/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package pulls

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/util/sets"

	github_util "k8s.io/contrib/mungegithub/github"
	"k8s.io/contrib/mungegithub/pulls/e2e"

	"github.com/golang/glog"
	github_api "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

const (
	needsOKToMergeLabel      = "needs-ok-to-merge"
	defaultJenkinsE2EContext = "jenkins/google"
)

var (
	_ = fmt.Print
)

type submitStatus struct {
	pullRequest
	Reason string
}

type pullRequest struct {
	Number    string
	URL       string
	Title     string
	Login     string
	AvatarURL string
}

type userInfo struct {
	AvatarURL string
	Access    string
}

type submitQueueStatus struct {
	PRStatus    map[string]submitStatus
	BuildStatus map[string]string
	UserInfo    map[string]userInfo
	E2ERunning  *pullRequest
	E2EQueue    []*pullRequest
}

// SubmitQueue will merge PR which meet a set of requirements.
//  PR must have LGTM after the last commit
//  PR must have passed all github CI checks
//  if user not in whitelist PR must have "ok-to-merge"
//  The google internal jenkins instance must be passing the JenkinsJobs e2e tests
type SubmitQueue struct {
	githubConfig           *github_util.Config
	JenkinsJobs            []string
	JenkinsHost            string
	Whitelist              string
	RequiredStatusContexts []string
	WhitelistOverride      string
	Committers             string
	Address                string
	DontRequireE2ELabel    string
	E2EStatusContext       string
	WWWRoot                string

	// additionalUserWhitelist are non-committer users believed safe
	additionalUserWhitelist *sets.String
	// CommitterList are static here in case they can't be gotten dynamically;
	// they do not need to be whitelisted.
	committerList *sets.String

	// userWhitelist is the combination of committers and additional which
	// we actully use
	userWhitelist *sets.String

	sync.Mutex
	lastPRStatus map[string]submitStatus
	prStatus     map[string]submitStatus // ALWAYS protected by sync.Mutex
	userInfo     map[string]userInfo

	// Every time a PR is added to githubE2EQueue also notify the channel
	githubE2EWakeup  chan bool
	githubE2ERunning *github_api.PullRequest
	githubE2EQueue   map[int]*github_api.PullRequest // protected by sync.Mutex!

	e2e *e2e.E2ETester
}

func init() {
	RegisterMungerOrDie(&SubmitQueue{})
}

// Name is the name usable in --pr-mungers
func (sq SubmitQueue) Name() string { return "submit-queue" }

// Initialize will initialize the munger
func (sq *SubmitQueue) Initialize(config *github_util.Config) error {
	sq.Lock()
	defer sq.Unlock()

	sq.githubConfig = config
	if len(sq.JenkinsHost) == 0 {
		glog.Fatalf("--jenkins-host is required.")
	}

	e2e := &e2e.E2ETester{
		JenkinsJobs: sq.JenkinsJobs,
		JenkinsHost: sq.JenkinsHost,
		BuildStatus: map[string]string{},
	}
	sq.e2e = e2e
	if len(sq.Address) > 0 {
		if len(sq.WWWRoot) > 0 {
			http.Handle("/", http.FileServer(http.Dir(sq.WWWRoot)))
		}
		http.Handle("/api", sq)
		go http.ListenAndServe(sq.Address, nil)
	}
	sq.prStatus = map[string]submitStatus{}
	sq.lastPRStatus = map[string]submitStatus{}

	sq.githubE2EWakeup = make(chan bool, 1000)
	sq.githubE2EQueue = map[int]*github_api.PullRequest{}

	go sq.handleGithubE2EAndMerge()
	go sq.updateGoogleE2ELoop()
	return nil
}

// EachLoop is called at the start of every munge loop
func (sq *SubmitQueue) EachLoop(config *github_util.Config) error {
	sq.Lock()
	defer sq.Unlock()
	sq.RefreshWhitelist(config)
	sq.lastPRStatus = sq.prStatus
	sq.prStatus = map[string]submitStatus{}
	return nil
}

// AddFlags will add any request flags to the cobra `cmd`
func (sq *SubmitQueue) AddFlags(cmd *cobra.Command, config *github_util.Config) {
	cmd.Flags().StringSliceVar(&sq.JenkinsJobs, "jenkins-jobs", []string{"kubernetes-e2e-gce", "kubernetes-e2e-gke-ci", "kubernetes-build", "kubernetes-e2e-gce-parallel", "kubernetes-e2e-gce-autoscaling", "kubernetes-e2e-gce-reboot", "kubernetes-e2e-gce-scalability"}, "Comma separated list of jobs in Jenkins to use for stability testing")
	cmd.Flags().StringVar(&sq.JenkinsHost, "jenkins-host", "http://jenkins-master:8080", "The URL for the jenkins job to watch")
	cmd.Flags().StringSliceVar(&sq.RequiredStatusContexts, "required-contexts", []string{"cla/google", "Shippable", "continuous-integration/travis-ci/pr"}, "Comma separate list of status contexts required for a PR to be considered ok to merge")
	cmd.Flags().StringVar(&sq.Address, "address", ":8080", "The address to listen on for HTTP Status")
	cmd.Flags().StringVar(&sq.DontRequireE2ELabel, "dont-require-e2e-label", "e2e-not-required", "If non-empty, a PR with this label will be merged automatically without looking at e2e results")
	cmd.Flags().StringVar(&sq.E2EStatusContext, "e2e-status-context", defaultJenkinsE2EContext, "The name of the github status context for the e2e PR Builder")
	cmd.Flags().StringVar(&sq.WWWRoot, "www", "www", "Path to static web files to serve from the webserver")
	sq.addWhitelistCommand(cmd, config)
}

// This serves little purpose other than to show updates every minute in the
// web UI. Stable() will get called as needed against individual PRs as well.
func (sq *SubmitQueue) updateGoogleE2ELoop() {
	for {
		if !sq.e2e.Stable() {
			sq.flushGithubE2EQueue(e2eFailure)
		}
		time.Sleep(1 * time.Minute)
	}

}

func prToStatusPullRequest(pr *github_api.PullRequest) *pullRequest {
	if pr == nil {
		return &pullRequest{}
	}
	return &pullRequest{
		Number:    strconv.Itoa(*pr.Number),
		URL:       *pr.HTMLURL,
		Title:     *pr.Title,
		Login:     *pr.User.Login,
		AvatarURL: *pr.User.AvatarURL,
	}
}

// SetPRStatus will set the status given a particular PR. This function should
// but used instead of manipulating the prStatus directly as sq.Lock() must be
// called when manipulating that structure
func (sq *SubmitQueue) SetPRStatus(pr *github_api.PullRequest, reason string) {
	num := strconv.Itoa(*pr.Number)
	submitStatus := submitStatus{
		pullRequest: *prToStatusPullRequest(pr),
		Reason:      reason,
	}
	sq.Lock()
	defer sq.Unlock()

	sq.prStatus[num] = submitStatus
	sq.cleanupOldE2E(pr, reason)
}

// sq.Lock() MUST be held!
func (sq *SubmitQueue) getE2EQueueStatus() []*pullRequest {
	queue := []*pullRequest{}
	keys := sq.orderedE2EQueue()
	for _, k := range keys {
		pr := sq.githubE2EQueue[k]
		request := prToStatusPullRequest(pr)
		queue = append(queue, request)
	}
	return queue
}

// GetQueueStatus returns a json representation of the state of the submit
// queue. This can be used to generate web pages about the submit queue.
func (sq *SubmitQueue) GetQueueStatus() []byte {
	status := submitQueueStatus{}
	sq.Lock()
	defer sq.Unlock()
	outputStatus := sq.lastPRStatus
	for key, value := range sq.prStatus {
		outputStatus[key] = value
	}
	status.PRStatus = outputStatus
	status.BuildStatus = sq.e2e.GetBuildStatus()
	status.UserInfo = sq.userInfo
	status.E2EQueue = sq.getE2EQueueStatus()
	status.E2ERunning = prToStatusPullRequest(sq.githubE2ERunning)

	b, err := json.Marshal(status)
	if err != nil {
		glog.Errorf("Unable to Marshal Status: %v", status)
		return nil
	}
	return b
}

var (
	unknown                 = "unknown failure"
	noCLA                   = "PR does not have cla: yes."
	noLGTM                  = "PR does not have LGTM."
	needsok                 = "PR does not have 'ok-to-merge' label"
	lgtmEarly               = "The PR was changed after the LGTM label was added."
	unmergeable             = "PR is unable to be automatically merged. Needs rebase."
	undeterminedMergability = "Unable to determine is PR is mergeable. Will try again later."
	ciFailure               = "Github CI tests are not green."
	e2eFailure              = "The e2e tests are failing. The entire submit queue is blocked."
	merged                  = "MERGED!"
	ghE2EQueued             = "Queued to run github e2e tests a second time."
	ghE2EWaitingStart       = "Requested and waiting for github e2e test to start running a second time."
	ghE2ERunning            = "Running github e2e tests a second time."
	ghE2EFailed             = "Second github e2e run failed."
)

// MungePullRequest is the workhorse the will actually make updates to the PR
func (sq *SubmitQueue) MungePullRequest(config *github_util.Config, pr *github_api.PullRequest, issue *github_api.Issue, commits []github_api.RepositoryCommit, events []github_api.IssueEvent) {
	e2e := sq.e2e
	userSet := sq.userWhitelist

	if !github_util.HasLabels(issue.Labels, []string{"cla: yes"}) {
		sq.SetPRStatus(pr, noCLA)
		return
	}

	if mergeable, err := config.IsPRMergeable(pr); err != nil {
		glog.V(2).Infof("Skipping %d - unable to determine mergeability", *pr.Number)
		sq.SetPRStatus(pr, undeterminedMergability)
		return
	} else if !mergeable {
		glog.V(4).Infof("Skipping %d - not mergable", *pr.Number)
		sq.SetPRStatus(pr, unmergeable)
		return
	}

	// Validate the status information for this PR
	contexts := sq.RequiredStatusContexts
	if len(sq.E2EStatusContext) > 0 && (len(sq.DontRequireE2ELabel) == 0 || !github_util.HasLabel(issue.Labels, sq.DontRequireE2ELabel)) {
		contexts = append(contexts, sq.E2EStatusContext)
	}
	if ok := config.IsStatusSuccess(pr, contexts); !ok {
		glog.Errorf("PR# %d Github CI status is not success", *pr.Number)
		sq.SetPRStatus(pr, ciFailure)
		return
	}

	if !github_util.HasLabel(issue.Labels, sq.WhitelistOverride) && !userSet.Has(*pr.User.Login) {
		glog.V(4).Infof("Dropping %d since %s isn't in whitelist and %s isn't present", *pr.Number, *pr.User.Login, sq.WhitelistOverride)
		if !github_util.HasLabel(issue.Labels, needsOKToMergeLabel) {
			config.AddLabels(*pr.Number, []string{needsOKToMergeLabel})
			body := "The author of this PR is not in the whitelist for merge, can one of the admins add the 'ok-to-merge' label?"
			config.WriteComment(*pr.Number, body)
		}
		sq.SetPRStatus(pr, needsok)
		return
	}

	// Tidy up the issue list.
	if github_util.HasLabel(issue.Labels, needsOKToMergeLabel) {
		config.RemoveLabel(*pr.Number, needsOKToMergeLabel)
	}

	if !github_util.HasLabels(issue.Labels, []string{"lgtm"}) {
		sq.SetPRStatus(pr, noLGTM)
		return
	}

	lastModifiedTime := github_util.LastModifiedTime(commits)
	lgtmTime := github_util.LabelTime("lgtm", events)

	if lastModifiedTime == nil || lgtmTime == nil {
		glog.Errorf("PR %d was unable to determine when LGTM was added or when last modified", *pr.Number)
		sq.SetPRStatus(pr, unknown)
		return
	}

	if lastModifiedTime.After(*lgtmTime) {
		glog.V(4).Infof("PR %d changed after LGTM. Will not merge", *pr.Number)
		sq.SetPRStatus(pr, lgtmEarly)
		return
	}

	if !e2e.Stable() {
		sq.flushGithubE2EQueue(e2eFailure)
		sq.SetPRStatus(pr, e2eFailure)
		return
	}

	// if there is a 'e2e-not-required' label, just merge it.
	if len(sq.DontRequireE2ELabel) > 0 && github_util.HasLabel(issue.Labels, sq.DontRequireE2ELabel) {
		config.MergePR(pr, "submit-queue")
		sq.SetPRStatus(pr, merged)
		return
	}

	sq.SetPRStatus(pr, ghE2EQueued)
	sq.Lock()
	sq.githubE2EWakeup <- true
	sq.githubE2EQueue[*pr.Number] = pr
	sq.Unlock()

	return
}

// If the PR was put in the github e2e queue on a current pass, but now we don't
// think it should be in the e2e queue, remove it. MUST be called with sq.Lock()
// held.
func (sq *SubmitQueue) cleanupOldE2E(pr *github_api.PullRequest, reason string) {
	switch reason {
	case ghE2EQueued:
	case ghE2EWaitingStart:
	case ghE2ERunning:
		// Do nothing
	default:
		delete(sq.githubE2EQueue, *pr.Number)
	}

}

// flushGithubE2EQueue will rmeove all entries from the build queue and will mark them
// as failed with the given reason. We do not need to flush the githubE2EWakeup
// channel as that just causes handleGithubE2EAndMerge() to wake up. And if it
// wakes up a few extra times, who cares.
func (sq *SubmitQueue) flushGithubE2EQueue(reason string) {
	prs := []*github_api.PullRequest{}
	sq.Lock()
	for _, pr := range sq.githubE2EQueue {
		prs = append(prs, pr)
	}
	sq.Unlock()
	for _, pr := range prs {
		sq.SetPRStatus(pr, reason)
	}
}

// sq.Lock() better held!!!
func (sq *SubmitQueue) orderedE2EQueue() []int {
	// Find and do the lowest PR number first
	var keys []int
	for k := range sq.githubE2EQueue {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// handleGithubE2EAndMerge waits for PRs that are ready to re-run the github
// e2e tests, runs the test, and then merges if everything was successful.
func (sq *SubmitQueue) handleGithubE2EAndMerge() {
	for {
		// Wait until something is ready to be processed
		select {
		case _ = <-sq.githubE2EWakeup:
		}

		sq.Lock()
		// Could happen if the same PR was added twice. It will only be
		// in the map one time, but it will be in the channel twice.
		if len(sq.githubE2EQueue) == 0 {
			sq.Unlock()
			continue
		}
		keys := sq.orderedE2EQueue()
		pr := sq.githubE2EQueue[keys[0]]
		sq.githubE2ERunning = pr
		sq.Unlock()

		// re-test and maybe merge
		sq.doGithubE2EAndMerge(pr)

		// remove it from the map after we finish testing
		sq.Lock()
		sq.githubE2ERunning = nil
		delete(sq.githubE2EQueue, keys[0])
		sq.Unlock()
	}
}

func (sq *SubmitQueue) doGithubE2EAndMerge(pr *github_api.PullRequest) {
	newpr, err := sq.githubConfig.GetPR(*pr.Number)
	if err != nil {
		sq.SetPRStatus(pr, unknown)
		return
	}
	pr = newpr

	if *pr.Merged {
		sq.SetPRStatus(pr, merged)
		return
	}

	if mergeable, err := sq.githubConfig.IsPRMergeable(pr); err != nil {
		sq.SetPRStatus(pr, undeterminedMergability)
		return
	} else if !mergeable {
		sq.SetPRStatus(pr, unmergeable)
		return
	}

	body := "@k8s-bot test this [submit-queue is verifying that this PR is safe to merge]"
	if err := sq.githubConfig.WriteComment(*pr.Number, body); err != nil {
		sq.SetPRStatus(pr, unknown)
		return
	}

	// Wait for the build to start
	sq.SetPRStatus(pr, ghE2EWaitingStart)
	err = sq.githubConfig.WaitForPending(pr)
	if err != nil {
		s := fmt.Sprintf("Failed waiting for PR to start testing: %v", err)
		sq.SetPRStatus(pr, s)
		return
	}

	// Wait for the status to go back to something other than pending
	sq.SetPRStatus(pr, ghE2ERunning)
	err = sq.githubConfig.WaitForNotPending(pr)
	if err != nil {
		s := fmt.Sprintf("Failed waiting for PR to finish testing: %v", err)
		sq.SetPRStatus(pr, s)
		return
	}

	// Check if the thing we care about is success
	if ok := sq.githubConfig.IsStatusSuccess(pr, []string{sq.E2EStatusContext}); !ok {
		glog.Infof("Status after build for %q is not 'success', skipping PR %d", sq.E2EStatusContext, *pr.Number)
		sq.SetPRStatus(pr, ghE2EFailed)
		return
	}

	if !sq.e2e.Stable() {
		sq.flushGithubE2EQueue(e2eFailure)
		sq.SetPRStatus(pr, e2eFailure)
		return
	}

	sq.githubConfig.MergePR(pr, "submit-queue")
	sq.SetPRStatus(pr, merged)
	return
}

func (sq *SubmitQueue) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	data := sq.GetQueueStatus()
	if data == nil {
		res.Header().Set("Content-type", "text/plain")
		res.WriteHeader(http.StatusInternalServerError)
	} else {
		res.Header().Set("Content-type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(data)
	}
}
