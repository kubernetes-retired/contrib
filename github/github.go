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

package github

import (
	goflag "flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"github.com/gregjones/httpcache"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

const (
	// stolen from https://groups.google.com/forum/#!msg/golang-nuts/a9PitPAHSSU/ziQw1-QHw3EJ
	maxInt = int(^uint(0) >> 1)
)

type RateLimitRoundTripper struct {
	delegate http.RoundTripper
	throttle util.RateLimiter
}

func (r *RateLimitRoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	r.throttle.Accept()
	return r.delegate.RoundTrip(req)
}

type GithubConfig struct {
	client  *github.Client
	Org     string
	Project string

	RateLimit      float32
	RateLimitBurst int

	Token     string
	TokenFile string

	MinPRNumber int
	MaxPRNumber int

	// If true, don't make any mutating API calls
	DryRun bool

	useMemoryCache bool

	analytics analytics
}

type analytic int

func (a *analytic) Call(config *GithubConfig) {
	config.analytics.apiCount++
	*a = *a + 1
}

type analytics struct {
	lastAPIReset time.Time
	apiCount     int // number of times we called a github API

	AddLabels         analytic
	RemoveLabels      analytic
	ListCollaborators analytic
	ListIssues        analytic
	ListIssueEvents   analytic
	ListCommits       analytic
	GetCommit         analytic
	GetCombinedStatus analytic
	GetPR             analytic
	AssignPR          analytic
	ClosePR           analytic
	OpenPR            analytic
	GetContents       analytic
	CreateComment     analytic
	Merge             analytic
}

func (a analytics) Print() {
	since := time.Since(a.lastAPIReset)
	callsPerSec := float64(a.apiCount) / since.Seconds()
	glog.Infof("Made %d API calls since the last Reset %f calls/sec", a.apiCount, callsPerSec)
	glog.V(2).Infof("Made %d AddLabels calls", a.AddLabels)
	glog.V(2).Infof("Made %d RemoveLabels calls", a.RemoveLabels)
	glog.V(2).Infof("Made %d ListCollaborators calls", a.ListCollaborators)
	glog.V(2).Infof("Made %d ListIssues calls", a.ListIssues)
	glog.V(2).Infof("Made %d ListIssueEvents calls", a.ListIssueEvents)
	glog.V(2).Infof("Made %d ListCommits calls", a.ListCommits)
	glog.V(2).Infof("Made %d GetCommit calls", a.GetCommit)
	glog.V(2).Infof("Made %d GetCombinedStatus calls", a.GetCombinedStatus)
	glog.V(2).Infof("Made %d GetPR calls", a.GetPR)
	glog.V(2).Infof("Made %d AssignPR calls", a.AssignPR)
	glog.V(2).Infof("Made %d ClosePR calls", a.ClosePR)
	glog.V(2).Infof("Made %d OpenPR calls", a.OpenPR)
	glog.V(2).Infof("Made %d GetContents calls", a.GetContents)
	glog.V(2).Infof("Made %d CreateComment calls", a.CreateComment)
	glog.V(2).Infof("Made %d Merge calls", a.Merge)
}

func (config *GithubConfig) AddRootFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&config.Token, "token", "", "The OAuth Token to use for requests.")
	cmd.PersistentFlags().StringVar(&config.TokenFile, "token-file", "", "The file containing the OAuth Token to use for requests.")
	cmd.PersistentFlags().IntVar(&config.MinPRNumber, "min-pr-number", 0, "The minimum PR to start with")
	cmd.PersistentFlags().IntVar(&config.MaxPRNumber, "max-pr-number", maxInt, "The maximum PR to start with")
	cmd.PersistentFlags().BoolVar(&config.DryRun, "dry-run", false, "If true, don't actually merge anything")
	cmd.PersistentFlags().BoolVar(&config.useMemoryCache, "use-http-cache", false, "If true, use a client side HTTP cache for API requests.")
	cmd.PersistentFlags().StringVar(&config.Org, "organization", "kubernetes", "The github organization to scan")
	cmd.PersistentFlags().StringVar(&config.Project, "project", "kubernetes", "The github project to scan")
	// Global limit is 5000 Q/Hour, try to only use 1800 to make room for other apps
	cmd.PersistentFlags().Float32Var(&config.RateLimit, "rate-limit", 1800, "Requests per hour we should allow")
	cmd.PersistentFlags().IntVar(&config.RateLimitBurst, "rate-limit-burst", 900, "Requests we allow to burst over the rate limit")
	cmd.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
}

func (config *GithubConfig) PreExecute() error {
	if len(config.Org) == 0 {
		glog.Fatalf("--organization is required.")
	}
	if len(config.Project) == 0 {
		glog.Fatalf("--project is required.")
	}

	token := config.Token
	if len(token) == 0 && len(config.TokenFile) != 0 {
		data, err := ioutil.ReadFile(config.TokenFile)
		if err != nil {
			glog.Fatalf("error reading token file: %v", err)
		}
		token = string(data)
	}

	transport := http.DefaultTransport
	if config.useMemoryCache {
		transport = httpcache.NewMemoryCacheTransport()
	}

	// convert from queries per hour to queries per second
	config.RateLimit = config.RateLimit / 3600
	// ignore the configured rate limit if you don't have a token.
	// only get 60 requests per hour!
	if len(token) == 0 {
		glog.Warningf("Ignoring --rate-limit because no token data available")
		config.RateLimit = 0.01
		config.RateLimitBurst = 10
	}
	rateLimitTransport := &RateLimitRoundTripper{
		delegate: transport,
		throttle: util.NewTokenBucketRateLimiter(config.RateLimit, config.RateLimitBurst),
	}

	client := &http.Client{
		Transport: rateLimitTransport,
	}
	if len(token) > 0 {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		client = &http.Client{
			Transport: &oauth2.Transport{
				Base:   rateLimitTransport,
				Source: oauth2.ReuseTokenSource(nil, ts),
			},
		}
	}
	config.client = github.NewClient(client)
	config.analytics.lastAPIReset = time.Now()
	return nil
}

func (config *GithubConfig) ResetAPICount() {
	config.analytics.Print()
	config.analytics = analytics{}
	config.analytics.lastAPIReset = time.Now()
}

// SetClient should ONLY be used by testing. Normal commands should use PreExecute()
func (config *GithubConfig) SetClient(client *github.Client) {
	config.client = client
}

func HasLabel(labels []github.Label, name string) bool {
	for i := range labels {
		label := &labels[i]
		if label.Name != nil && *label.Name == name {
			return true
		}
	}
	return false
}

func HasLabels(labels []github.Label, names []string) bool {
	for i := range names {
		if !HasLabel(labels, names[i]) {
			return false
		}
	}
	return true
}

func GetLabelsWithPrefix(labels []github.Label, prefix string) []string {
	var ret []string
	for _, label := range labels {
		if label.Name != nil && strings.HasPrefix(*label.Name, prefix) {
			ret = append(ret, *label.Name)
		}
	}
	return ret
}

func (config *GithubConfig) AddLabels(prNum int, labels []string) error {
	config.analytics.AddLabels.Call(config)
	if config.DryRun {
		glog.Infof("Would have added labels %v to PR %d but --dry-run is set", labels, prNum)
		return nil
	}
	if _, _, err := config.client.Issues.AddLabelsToIssue(config.Org, config.Project, prNum, labels); err != nil {
		glog.Errorf("Failed to set labels %v for %d: %v", labels, prNum, err)
		return err
	}
	return nil
}

func (config *GithubConfig) RemoveLabel(prNum int, label string) error {
	config.analytics.RemoveLabels.Call(config)
	if config.DryRun {
		glog.Infof("Would have removed label %q to PR %d but --dry-run is set", label, prNum)
		return nil
	}
	if _, err := config.client.Issues.RemoveLabelForIssue(config.Org, config.Project, prNum, label); err != nil {
		glog.Errorf("Failed to remove %d from issue %d: %v", label, prNum, err)
		return err
	}
	return nil
}

type PRFunction func(*github.PullRequest, *github.Issue) error
type IssueFunction func(*github.Issue) error

func (config *GithubConfig) LastModifiedTime(prNum int) (*time.Time, error) {
	config.analytics.ListCommits.Call(config)
	list, _, err := config.client.PullRequests.ListCommits(config.Org, config.Project, prNum, &github.ListOptions{})
	if err != nil {
		return nil, err
	}
	var lastModified *time.Time
	for ix := range list {
		item := list[ix]
		if lastModified == nil || item.Commit.Committer.Date.After(*lastModified) {
			lastModified = item.Commit.Committer.Date
		}
	}
	return lastModified, nil
}

func (config *GithubConfig) fetchAllCollaborators() ([]github.User, error) {
	page := 1
	var result []github.User
	for {
		glog.V(4).Infof("Fetching page %d of all users", page)
		config.analytics.ListCollaborators.Call(config)
		listOpts := &github.ListOptions{PerPage: 100, Page: page}
		users, response, err := config.client.Repositories.ListCollaborators(config.Org, config.Project, listOpts)
		if err != nil {
			return nil, err
		}
		result = append(result, users...)
		if response.LastPage == 0 || response.LastPage == page {
			break
		}
		page++
	}
	return result, nil
}

// UsersWithAccess returns two sets of users. The first set are users with push
// access. The second set is the specific set of user with pull access. If the
// repo is public all users will have pull access, but some with have it
// explicitly
func (config *GithubConfig) UsersWithAccess() (pushUsers sets.String, pullUsers sets.String, err error) {
	pushUsers = sets.String{}
	pullUsers = sets.String{}

	users, err := config.fetchAllCollaborators()
	if err != nil {
		glog.Errorf("%v", err)
		return nil, nil, err
	}

	for _, user := range users {
		if user.Permissions == nil || user.Login == nil {
			err := fmt.Errorf("Found a user with nil Permissions or Login")
			glog.Errorf("%v", err)
			return nil, nil, err
		}
		login := *user.Login
		perms := *user.Permissions
		if perms["push"] {
			pushUsers.Insert(login)
		} else if perms["pull"] {
			pullUsers.Insert(login)
		}
	}
	return pushUsers, pullUsers, nil
}

func (config *GithubConfig) GetAllEventsForPR(prNum int) ([]github.IssueEvent, error) {
	events := []github.IssueEvent{}
	page := 1
	for {
		config.analytics.ListIssueEvents.Call(config)
		eventPage, response, err := config.client.Issues.ListIssueEvents(config.Org, config.Project, prNum, &github.ListOptions{Page: page})
		if err != nil {
			glog.Errorf("Error getting events for issue: %v", err)
			return nil, err
		}
		events = append(events, eventPage...)
		if response.LastPage == 0 || response.LastPage == page {
			break
		}
		page++
	}
	return events, nil
}

func (config *GithubConfig) getCommitStatus(prNum int) ([]*github.CombinedStatus, error) {
	config.analytics.ListCommits.Call(config)
	commits, _, err := config.client.PullRequests.ListCommits(config.Org, config.Project, prNum, &github.ListOptions{})
	if err != nil {
		return nil, err
	}
	commitStatus := make([]*github.CombinedStatus, len(commits))
	for ix := range commits {
		commit := &commits[ix]
		config.analytics.GetCombinedStatus.Call(config)
		statusList, _, err := config.client.Repositories.GetCombinedStatus(config.Org, config.Project, *commit.SHA, &github.ListOptions{})
		if err != nil {
			return nil, err
		}
		commitStatus[ix] = statusList
	}
	return commitStatus, nil
}

// Gets the current status of a PR by introspecting the status of the commits in the PR.
// The rules are:
//    * If any member of the 'requiredContexts' list is missing, it is 'incomplete'
//    * If any commit is 'pending', the PR is 'pending'
//    * If any commit is 'error', the PR is in 'error'
//    * If any commit is 'failure', the PR is 'failure'
//    * Otherwise the PR is 'success'
func (config *GithubConfig) GetStatus(prNum int, requiredContexts []string) (string, error) {
	statusList, err := config.getCommitStatus(prNum)
	if err != nil {
		return "", err
	}
	return computeStatus(statusList, requiredContexts), nil
}

func computeStatus(statusList []*github.CombinedStatus, requiredContexts []string) string {
	states := sets.String{}
	providers := sets.String{}
	for ix := range statusList {
		status := statusList[ix]
		glog.V(8).Infof("Checking commit: %s status:%v", *status.SHA, status)
		states.Insert(*status.State)

		for _, subStatus := range status.Statuses {
			glog.V(8).Infof("Found status from: %v", subStatus)
			providers.Insert(*subStatus.Context)
		}
	}
	for _, provider := range requiredContexts {
		if !providers.Has(provider) {
			glog.V(8).Infof("Failed to find %q in %v", provider, providers)
			return "incomplete"
		}
	}

	switch {
	case states.Has("pending"):
		return "pending"
	case states.Has("error"):
		return "error"
	case states.Has("failure"):
		return "failure"
	default:
		return "success"
	}
}

// Make sure that the combined status for all commits in a PR is 'success'
// if 'waitForPending' is true, this function will wait until the PR is no longer pending (all checks have run)
func (config *GithubConfig) ValidateStatus(prNum int, requiredContexts []string, waitOnPending bool) (bool, error) {
	pending := true
	for pending {
		status, err := config.GetStatus(prNum, requiredContexts)
		if err != nil {
			return false, err
		}
		switch status {
		case "error", "failure":
			return false, nil
		case "pending":
			if !waitOnPending {
				return false, nil
			}
			pending = true
			glog.V(4).Info("PR is pending, waiting for 30 seconds")
			time.Sleep(30 * time.Second)
		case "success":
			return true, nil
		case "incomplete":
			return false, nil
		default:
			return false, fmt.Errorf("unknown status: %q", status)
		}
	}
	return true, nil
}

// Wait for a PR to move into Pending.  This is useful because the request to test a PR again
// is asynchronous with the PR actually moving into a pending state
// TODO: add a timeout
func (config *GithubConfig) WaitForPending(prNum int) error {
	for {
		status, err := config.GetStatus(prNum, []string{})
		if err != nil {
			return err
		}
		if status == "pending" {
			return nil
		}
		glog.V(4).Info("PR is not pending, waiting for 30 seconds")
		time.Sleep(30 * time.Second)
	}
}

func (config *GithubConfig) GetCommits(prNum int) ([]github.RepositoryCommit, error) {
	config.analytics.ListCommits.Call(config)
	//TODO: this should handle paging, I believe....
	commits, _, err := config.client.PullRequests.ListCommits(config.Org, config.Project, prNum, &github.ListOptions{})
	if err != nil {
		return nil, err
	}
	return commits, nil
}

func (config *GithubConfig) GetFilledCommits(prNum int) ([]github.RepositoryCommit, error) {
	commits, err := config.GetCommits(prNum)
	if err != nil {
		return nil, err
	}
	filledCommits := []github.RepositoryCommit{}
	for _, c := range commits {
		config.analytics.GetCommit.Call(config)
		commit, _, err := config.client.Repositories.GetCommit(config.Org, config.Project, *c.SHA)
		if err != nil {
			glog.Errorf("Can't load commit %s %s %s", config.Org, config.Project, *commit.SHA)
			continue
		}
		filledCommits = append(filledCommits, *commit)
	}
	return filledCommits, nil
}

func (config *GithubConfig) GetPR(prNum int) (*github.PullRequest, error) {
	config.analytics.GetPR.Call(config)
	pr, _, err := config.client.PullRequests.Get(config.Org, config.Project, prNum)
	if err != nil {
		glog.Errorf("Error getting PR# %d: %v", prNum, err)
		return nil, err
	}
	return pr, nil
}

func (config *GithubConfig) AssignPR(prNum int, owner string) error {
	config.analytics.AssignPR.Call(config)
	assignee := &github.IssueRequest{Assignee: &owner}
	if config.DryRun {
		glog.Infof("Would have assigned PR# %d  to %v but --dry-run was set", prNum, owner)
		return nil
	}
	if _, _, err := config.client.Issues.Edit(config.Org, config.Project, prNum, assignee); err != nil {
		glog.Errorf("Error assigning issue# %d to %v: %v", prNum, owner, err)
		return err
	}
	return nil
}

func (config *GithubConfig) ClosePR(pr *github.PullRequest) error {
	config.analytics.ClosePR.Call(config)
	if config.DryRun {
		glog.Infof("Would have closed PR# %d but --dry-run was set", *pr.Number)
		return nil
	}
	state := "closed"
	pr.State = &state
	if _, _, err := config.client.PullRequests.Edit(config.Org, config.Project, *pr.Number, pr); err != nil {
		glog.Errorf("Failed to close pr %d: %v", *pr.Number, err)
		return err
	}
	return nil
}

// OpenPR will attempt to open the given PR.
func (config *GithubConfig) OpenPR(pr *github.PullRequest, numTries int) error {
	config.analytics.OpenPR.Call(config)
	if config.DryRun {
		glog.Infof("Would have openned PR# %d but --dry-run was set", *pr.Number)
		return nil
	}
	var err error
	state := "open"
	pr.State = &state
	// Try pretty hard to re-open, since it's pretty bad if we accidentally leave a PR closed
	for tries := 0; tries < numTries; tries++ {
		if _, _, err = config.client.PullRequests.Edit(config.Org, config.Project, *pr.Number, pr); err != nil {
			return nil
		}
		glog.Warningf("failed to re-open pr %d: %v", *pr.Number, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		glog.Errorf("failed to re-open pr %d after %d tries, giving up: %v", *pr.Number, numTries, err)
	}
	return err
}

func (config *GithubConfig) GetFileContents(file, sha string) (string, error) {
	config.analytics.GetContents.Call(config)
	getOpts := &github.RepositoryContentGetOptions{Ref: sha}
	output, _, _, err := config.client.Repositories.GetContents(config.Org, config.Project, file, getOpts)
	if err != nil {
		err = fmt.Errorf("Unable to get %q at commit %s", file, sha)
		// I'm using .V(2) because .generated docs is still not in the repo...
		glog.V(2).Infof("%v", err)
		return "", err
	}
	if output == nil {
		err = fmt.Errorf("Got empty contents for %q at commit %s", file, sha)
		glog.Errorf("%v", err)
		return "", err
	}
	b, err := output.Decode()
	if err != nil {
		glog.Errorf("Unable to decode file contents: %v", err)
		return "", err
	}
	return string(b), nil
}

// MergePR will merge the given PR, duh
// "who" is who is doing the merging, like "submit-queue"
func (config *GithubConfig) MergePR(prNum int, who string) error {
	config.analytics.Merge.Call(config)
	if config.DryRun {
		glog.Infof("Would have merged %d but --dry-run is set", prNum)
		return nil
	}
	glog.Infof("Merging PR# %d", prNum)
	mergeBody := "Automatic merge from " + who
	config.WriteComment(prNum, mergeBody)
	if _, _, err := config.client.PullRequests.Merge(config.Org, config.Project, prNum, "Auto commit by PR queue bot"); err != nil {
		glog.Errorf("Failed to create merge comment: %v", err)
		return err
	}
	return nil
}

// WriteComment will send the `msg` as a comment to the specified PR
func (config *GithubConfig) WriteComment(prNum int, msg string) error {
	config.analytics.CreateComment.Call(config)
	if config.DryRun {
		glog.Infof("Would have commented %q in %d but --dry-run is set", msg, prNum)
		return nil
	}
	glog.Infof("Adding comment: %q to PR %d", msg, prNum)
	if _, _, err := config.client.Issues.CreateComment(config.Org, config.Project, prNum, &github.IssueComment{Body: &msg}); err != nil {
		glog.Errorf("%v", err)
		return err
	}
	return nil
}

// IsPRMergeable will return if the PR is mergeable. It will pause and get the
// PR again if github did not respond the first time. So the hopefully github
// will have a response the second time. If we have no answer twice, we return
// false
func (config *GithubConfig) IsPRMergeable(pr *github.PullRequest) (bool, error) {
	if pr.Mergeable == nil {
		var err error
		glog.Infof("Waiting for mergeability on %q %d", *pr.Title, *pr.Number)
		// TODO: determine what a good empirical setting for this is.
		time.Sleep(2 * time.Second)
		pr, err = config.GetPR(*pr.Number)
		if err != nil {
			glog.Errorf("Unable to get PR# %d: %v", *pr.Number, err)
			return false, err
		}
	}
	if pr.Mergeable == nil {
		err := fmt.Errorf("No mergeability information for %q %d, Skipping.", *pr.Title, *pr.Number)
		glog.Errorf("%v", err)
		return false, err
	}
	if !*pr.Mergeable {
		return false, nil
	}
	return true, nil

}

// For each Issue in the project that matches:
//   * pr.Number >= minPRNumber
//   * pr.Number <= maxPRNumber
//   * all labels are on the PR
// Run the specified function
func (config *GithubConfig) forEachIssueDo(labels []string, fn IssueFunction) error {
	page := 1
	for {
		glog.V(4).Infof("Fetching page %d of issues", page)
		config.analytics.ListIssues.Call(config)
		listOpts := &github.IssueListByRepoOptions{
			Sort:        "created",
			Labels:      labels,
			State:       "open",
			ListOptions: github.ListOptions{PerPage: 20, Page: page},
		}
		issues, response, err := config.client.Issues.ListByRepo(config.Org, config.Project, listOpts)
		if err != nil {
			return err
		}
		for i := range issues {
			issue := &issues[i]
			if issue.Number == nil {
				glog.Infof("Skipping issue with no number, very strange")
				continue
			}
			if issue.User == nil || issue.User.Login == nil {
				glog.V(2).Infof("Skipping PR %d with no user info %#v.", *issue.Number, issue.User)
				continue
			}
			if *issue.Number < config.MinPRNumber {
				glog.V(6).Infof("Dropping %d < %d", *issue.Number, config.MinPRNumber)
				continue
			}
			if *issue.Number > config.MaxPRNumber {
				glog.V(6).Infof("Dropping %d > %d", *issue.Number, config.MaxPRNumber)
				continue
			}
			glog.V(8).Infof("Issue %d labels: %v isPR: %v", *issue.Number, issue.Labels, issue.PullRequestLinks == nil)
			glog.V(8).Infof("%v", issue.Labels)
			if err := fn(issue); err != nil {
				return err
			}
		}
		if response.LastPage == 0 || response.LastPage == page {
			break
		}
		page++
	}
	return nil
}

func (config *GithubConfig) ForEachIssueDo(labels []string, fn IssueFunction) error {
	handleIssue := func(issue *github.Issue) error {
		if issue.PullRequestLinks != nil {
			return nil
		}
		glog.V(2).Infof("----==== %d ====----", *issue.Number)

		return fn(issue)
	}
	return config.forEachIssueDo(labels, handleIssue)
}

func (config *GithubConfig) ForEachPRDo(labels []string, fn PRFunction) error {
	handlePR := func(issue *github.Issue) error {
		if issue.PullRequestLinks == nil {
			return nil
		}
		pr, err := config.GetPR(*issue.Number)
		if err != nil {
			return err
		}
		if pr.Merged != nil && *pr.Merged {
			glog.V(3).Infof("PR %d was merged, may want to reduce the PerPage so this happens less often", *issue.Number)
			return nil
		}
		glog.V(2).Infof("----==== %d ====----", *issue.Number)

		if pr.Mergeable == nil {
			glog.V(2).Infof("Waiting for mergeability on %q %d", *pr.Title, *pr.Number)
			time.Sleep(2 * time.Second)
			pr, err = config.GetPR(*pr.Number)
			if err != nil {
				return err
			}
			if pr.Mergeable == nil {
				glog.Infof("No mergeability for PR %d after pause. Maybe increase pause time?", *pr.Number)
			}
		}
		return fn(pr, issue)
	}
	return config.forEachIssueDo(labels, handlePR)
}
