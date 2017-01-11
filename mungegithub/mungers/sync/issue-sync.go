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

package sync

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	// BotName is the name of merge-bot
	BotName = "k8s-merge-robot"
	// JenkinsBotName is the name of kubekins bot
	JenkinsBotName = "k8s-bot"
	priorityPrefix = "priority/P"
	// PriorityP0 represents Priority P0
	PriorityP0 = Priority(0)
	// PriorityP1 represents Priority P1
	PriorityP1 = Priority(1)
	// PriorityP2 represents Priority P2
	PriorityP2 = Priority(2)
	// PriorityP3 represents Priority P3
	PriorityP3 = Priority(3)
)

// RobotUser is a set of name of robot user
var RobotUser = sets.NewString(JenkinsBotName, BotName)

// Priority represents the priority label in an issue
type Priority int

// String return the priority label in string
func (p Priority) String() string {
	return fmt.Sprintf(priorityPrefix+"%d", p)
}

// Priority returns the priority in int
func (p Priority) Priority() int {
	return int(p)
}

// OwnerMapper finds an owner for a given test name.
type OwnerMapper interface {
	// TestOwner returns a GitHub username for a test, or "" if none are found.
	TestOwner(testName string) string
}

// IssueFinder finds an issue for a given key.
type IssueFinder interface {
	AllIssuesForKey(key string) []int
	Created(key string, number int)
}

// IssueSource can be implemented by anything that wishes to be synced with
// github issues.
type IssueSource interface {
	// Title is used to identify issues, so you must never change the
	// mechanism or you'll get duplicates.
	Title() string

	// If ID() is found in either the body of the issue or the body of any
	// of its comments, then a new comment doesn't need to be made. A URL
	// to more details is a good choice.
	// Additionally, ID is used to tell if we've already successfully
	// synced a given source. So it must be unique for every source.
	ID() string

	// If a new issue or comment must be made, Body is called to get the
	// text. Body *must* contain the output of ID().
	// newIssue will be true if we are starting a new issue, otherwise we
	// are adding a comment to an existing issue.
	Body(newIssue bool) string

	// AddTo attempts to merge Body into the output of another IssueSource's Body.
	// If this is sensible and valid, it returns the new body. An empty string indicates
	// a failure to merge the two.
	AddTo(previous string) string

	// If an issue is filed, these labels will be applied.
	Labels() []string

	// Priority calculates and returns the priority of an flake issue
	Priority(obj *github.MungeObject) (Priority, bool)
}

// IssueSyncer implements robust issue syncing logic and won't file duplicates etc.
type IssueSyncer struct {
	config *github.Config
	finder IssueFinder
	synced sets.String
	owner  OwnerMapper // 'owner' may be nil, disabling issue assignment.
}

// NewIssueSyncer constructs an issue syncer.
func NewIssueSyncer(config *github.Config, finder IssueFinder, owner OwnerMapper) *IssueSyncer {
	return &IssueSyncer{
		config: config,
		finder: finder,
		synced: sets.NewString(),
		owner:  owner,
	}
}

// Sync syncs the issue. It is fine and cheap to call Sync repeatedly for the
// same source.
func (s *IssueSyncer) Sync(source IssueSource) error {
	if s.synced.Has(source.ID()) {
		return nil
	}

	found, updatableIssues, ok := s.findPreviousIssues(source)
	if !ok {
		return fmt.Errorf("Unable to find PreviousIssues for %v", source)
	}

	// Close dups if there are multiple open issues
	if len(updatableIssues) > 1 {
		obj := updatableIssues[0]
		if err := s.markAsDups(updatableIssues[1:], *obj.Issue.Number); err != nil {
			return err
		}
	}

	if found {
		// Don't need to update, we were only here to close the dups.
		s.synced.Insert(source.ID())
		return nil
	}

	var obj *github.MungeObject
	// Update an issue if possible.
	if len(updatableIssues) > 0 {
		obj = updatableIssues[0]
		// Update the chosen issue
		if err := s.updateIssue(obj, source); err != nil {
			return fmt.Errorf("error updating issue %v for %v: %v", *obj.Issue.Number, source.ID(), err)
		}
		s.synced.Insert(source.ID())
		return nil
	}

	// No issue could be updated, create a new issue.
	obj, err := s.createIssue(source)
	if err != nil {
		return fmt.Errorf("error making issue for %v: %v", source.ID, err)
	}
	issueNum := *obj.Issue.Number
	s.finder.Created(source.Title(), issueNum)
	s.synced.Insert(source.ID())
	return nil
}

// Look through all issues filed about this item.
// If foundIn is > 0, then the particular item was found in that issue.
// All open issues for this item are returned in updatableIssues.
func (s *IssueSyncer) findPreviousIssues(source IssueSource) (found bool, updatableIssues []*github.MungeObject, ok bool) {
	possibleIssues := s.finder.AllIssuesForKey(source.Title())
	for _, previousIssue := range possibleIssues {
		obj, err := s.config.GetObject(previousIssue)
		if err != nil {
			return false, nil, false
		}
		isRecorded, ok := s.isRecorded(obj, source)
		if !ok {
			return false, nil, false
		}
		if isRecorded {
			found = true
			// keep going since we may want to close dups
		}
		if obj.Issue.State != nil && *obj.Issue.State == "open" {
			updatableIssues = append(updatableIssues, obj)
		}
	}
	return found, updatableIssues, true
}

// Close all of the dups.
func (s *IssueSyncer) markAsDups(dups []*github.MungeObject, of int) error {
	// Somehow we got duplicate issues all open at once.
	// Close all of the older ones.
	for _, dup := range dups {
		if err := dup.CloseIssuef("This is a duplicate of #%v; closing", of); err != nil {
			return fmt.Errorf("failed to close %v as a dup of %v: %v", *dup.Issue.Number, of, err)
		}
	}
	return nil
}

// Search through the body and comments to see if the given item is already
// mentioned in the given github issue.
func (s *IssueSyncer) isRecorded(obj *github.MungeObject, source IssueSource) (bool, bool) {
	id := source.ID()
	if obj.Issue.Body != nil && strings.Contains(*obj.Issue.Body, id) {
		// We already wrote this item
		return true, true
	}
	comments, ok := obj.ListComments()
	if !ok {
		return false, false
	}
	for _, c := range comments {
		if c.Body == nil {
			continue
		}
		if strings.Contains(*c.Body, id) {
			// We already wrote this item
			return true, true
		}
	}
	return false, true
}

// updateIssue adds a comment about the item to the github issue, or updates a comment.
func (s *IssueSyncer) updateIssue(obj *github.MungeObject, source IssueSource) error {
	body := source.Body(false)
	id := source.ID()
	if !strings.Contains(body, source.ID()) {
		// prevent making tons of duplicate comments
		panic(fmt.Errorf("Programmer error: %v does not contain %v!", body, id))
	}

	// Try to update an existing comment.
	// It will only update the last comment for this failure.
	// It will not update a comment if someone else has commented after it.
	// It will not update a comment more than 2 weeks old.
	comments, ok := obj.ListComments()
	if !ok {
		return fmt.Errorf("error getting comments for %v", *obj.Issue.Number)
	}
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		if c.User == nil || c.User.Login == nil || *c.User.Login != BotName {
			break
		}
		if c.CreatedAt != nil && c.CreatedAt.Before(time.Now().AddDate(0, 0, -14)) {
			break
		}
		if c.Body == nil {
			continue
		}
		combined := source.AddTo(*c.Body)
		if len(combined) > 65000 {
			glog.Infof("Not editing comment in issue %v because it would be too long (%dB)", *obj.Issue.Number, len(combined))
			continue
		}
		if combined != "" {
			glog.Infof("Editing comment in issue %v to add item %v", *obj.Issue.Number, source.ID())
			return obj.EditComment(c, combined)
		}
	}

	glog.Infof("Writing comment on issue %v with item %v", *obj.Issue.Number, source.ID())
	if err := obj.WriteComment(body); err != nil {
		return err
	}
	p, ok := source.Priority(obj)
	if !ok {
		return fmt.Errorf("Unable to get priority")
	}
	return s.syncPriority(obj, p)
}

func combineIssueComments(current, extra string) {

}

// createIssue makes a new issue for the given item. If we know about other
// issues for the item, then they'll be referenced.
func (s *IssueSyncer) createIssue(source IssueSource) (*github.MungeObject, error) {
	body := source.Body(true)
	id := source.ID()
	if !strings.Contains(body, source.ID()) {
		// prevent making tons of duplicate comments
		panic(fmt.Errorf("Programmer error: %v does not contain %v!", body, id))
	}

	var owner string
	if s.owner != nil {
		owner = s.owner.TestOwner(source.Title())
	}

	obj, err := s.config.NewIssue(
		source.Title(),
		body,
		source.Labels(),
		owner,
	)
	if err != nil {
		return nil, err
	}
	glog.Infof("Created issue %v:\n%v", *obj.Issue.Number, body)
	return obj, nil
}

// syncPriority will sync the input priority to the issue if the input priority is higher than the existing ones
func (s *IssueSyncer) syncPriority(obj *github.MungeObject, priority Priority) error {
	if obj.Priority() <= priority.Priority() {
		return nil
	}
	plabels := github.GetLabelsWithPrefix(obj.Issue.Labels, priorityPrefix)
	err := obj.AddLabel(priority.String())
	if err != nil {
		return nil
	}
	for _, l := range plabels {
		err = obj.RemoveLabel(l)
		if err != nil {
			return err
		}
	}
	return nil
}
