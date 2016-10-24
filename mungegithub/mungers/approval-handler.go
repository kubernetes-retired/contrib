/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mungers

import (
	"strings"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	mungeComment "k8s.io/contrib/mungegithub/mungers/matchers/comment"
	"k8s.io/contrib/mungegithub/mungers/mungerutil"
	"k8s.io/kubernetes/pkg/util/sets"

	"bytes"
	"fmt"
	"github.com/golang/glog"
	goGithub "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"sort"
)

const (
	approvalNotificationName = "ApprovalNotifier"
)

// ApprovalHandler will try to add "approved" label once
// all files of change has been approved by approvers.
type ApprovalHandler struct {
	features *features.Features
}

func init() {
	h := &ApprovalHandler{}
	RegisterMungerOrDie(h)
}

// Name is the name usable in --pr-mungers
func (*ApprovalHandler) Name() string { return "approval-handler" }

// RequiredFeatures is a slice of 'features' that must be provided
func (*ApprovalHandler) RequiredFeatures() []string {
	return []string{features.RepoFeatureName, features.AliasesFeature}
}

// Initialize will initialize the munger
func (h *ApprovalHandler) Initialize(config *github.Config, features *features.Features) error {
	h.features = features
	return nil
}

// EachLoop is called at the start of every munge loop
func (*ApprovalHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (*ApprovalHandler) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
// The algorithm goes as:
// - Initially, we build an approverSet
//   - Go through all comments after latest commit.
//	- If anyone said "/approve", add them to approverSet.
// - Then, for each file, we see if any approver of this file is in approverSet and keep track of files without approval
//   - An approver of a file is defined as:
//     - Someone listed as an "approver" in an OWNERS file in the files directory OR
//     - in one of the file's parent directorie
// - Iff all files have been approved, the bot will add the "approved" label.
// - Iff the approved label has been added and a cancel command is found, that reviewer will be removed from the approverSet
// 	and the munger will remove the approved label
func (h *ApprovalHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	files, err := obj.ListFiles()
	if err != nil {
		glog.Errorf("failed to list files in this PR: %v", err)
		return
	}

	comments, err := getCommentsAfterLastModified(obj)
	if err != nil {
		glog.Errorf("failed to get comments in this PR: %v", err)
		return
	}

	approverSet := createApproverSet(comments)
	needsApproval := h.getApprovalNeededFiles(files, approverSet)

	if err := updateNotification(obj, needsApproval); err != nil {
		return
	}
	if needsApproval.Len() > 0 {
		if obj.HasLabel(approvedLabel) {
			glog.Infof("Canceling the approve label because these files %v have no valid approvers", needsApproval)
			obj.RemoveLabel(approvedLabel)
		}
	} else if !obj.HasLabel(approvedLabel) {
		obj.AddLabel(approvedLabel)
	}
}

func updateNotification(obj *github.MungeObject, needsApproval sets.String) error {
	notificationMatcher := mungeComment.MungerNotificationName(approvalNotificationName)
	comments, err := obj.ListComments()
	if err != nil {
		glog.Error("Could not list the comments for PR%v", obj.Issue.Number)
		return err
	}
	latestNotification := mungeComment.FilterComments(comments, notificationMatcher).GetLast()
	latestApprove := mungeComment.FilterComments(comments, mungeComment.CommandName("approve")).GetLast()
	if latestNotification == nil {
		return createMessage(obj, needsApproval)
	}
	if latestApprove == nil {
		// there was already a bot notification and nothing has changed since
		return nil
	}
	if latestApprove.CreatedAt.After(*latestNotification.CreatedAt) {
		// there has been approval since last notification
		obj.DeleteComment(latestNotification)
		return createMessage(obj, needsApproval)
	}
	lastModified := obj.LastModifiedTime()
	if latestNotification.CreatedAt.Before(*lastModified) {
		obj.DeleteComment(latestNotification)
		return createMessage(obj, needsApproval)
	}
	return nil
}

func createMessage(obj *github.MungeObject, filesNeedApproval sets.String) error {
	files, err := obj.ListFiles()
	if err != nil {
		glog.Error("Could not list the files for PR%v", obj.Issue.Number)
		return err
	}
	approvedFiles := sets.String{}
	for _, fn := range files {
		if !filesNeedApproval.Has(*fn.Filename) {
			approvedFiles.Insert(*fn.Filename)
		}
	}
	approvedList := approvedFiles.List()
	sort.Strings(approvedList)
	needsApprovalList := filesNeedApproval.List()
	sort.Strings(needsApprovalList)

	context := bytes.NewBufferString("The following files have been approved:\n")
	fileFmt := "- %s\n"
	for _, fn := range approvedList {
		context.WriteString(fmt.Sprintf(fileFmt, fn))
	}
	context.WriteString("\n")
	context.WriteString("The following files REQUIRE approval:\n")
	for _, fn := range needsApprovalList {
		context.WriteString(fmt.Sprintf(fileFmt, fn))
	}
	context.WriteString("\n")
	return mungeComment.Notification{approvalNotificationName, "", context.String()}.Post(obj)
}

// createApproverSet iterates through the list of comments on a PR
// and identifies all of the people that have said /approve and adds
// them to the approverSet.  The function uses the latest approve or cancel comment
// to determine the Users intention
func createApproverSet(comments []*goGithub.IssueComment) sets.String {
	approverSet := sets.String{}
	for _, c := range comments {
		if !mungerutil.IsValidUser(c.User) {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(*c.Body))

		if len(fields) == 1 && strings.ToLower(fields[0]) == "/approve" {
			approverSet.Insert(*c.User.Login)
		} else if len(fields) == 2 && strings.ToLower(fields[0]) == "/approve" && strings.ToLower(fields[1]) == "cancel" {
			if approverSet.Has(*c.User.Login) {
				approverSet.Delete(*c.User.Login)
			}
		}
	}
	return approverSet
}

// getApprovalNeededFiles identifies the files that still need approval from someone in their OWNERS files
func (h ApprovalHandler) getApprovalNeededFiles(files []*goGithub.CommitFile, approverSet sets.String) sets.String {
	needsApproval := sets.String{}
	for _, file := range files {
		if !h.isApproved(file, approverSet) {
			needsApproval.Insert(*file.Filename)
		}
	}
	return needsApproval
}

// isApproved indicates whether or not someone from the list of OWNERS for a file has approved the PR
func (h ApprovalHandler) isApproved(someFile *goGithub.CommitFile, approverSet sets.String) bool {
	fileOwners := h.features.Repos.Assignees(*someFile.Filename)
	return fileOwners.Intersection(approverSet).Len() > 0
}

func getCommentsAfterLastModified(obj *github.MungeObject) ([]*goGithub.IssueComment, error) {
	afterLastModified := func(opt *goGithub.IssueListCommentsOptions) *goGithub.IssueListCommentsOptions {
		// Only comments updated at or after this time are returned.
		// One possible case is that reviewer might "/lgtm" first, contributor updated PR, and reviewer updated "/lgtm".
		// This is still valid. We don't recommend user to update it.
		lastModified := *obj.LastModifiedTime()
		opt.Since = lastModified
		return opt
	}
	return obj.ListComments(afterLastModified)
}
