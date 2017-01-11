/*
Copyright 2015 The Kubernetes Authors.

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
	"time"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/features"
	mgh "k8s.io/contrib/mungegithub/github"
	c "k8s.io/contrib/mungegithub/mungers/matchers/comment"
	"k8s.io/contrib/mungegithub/mungers/mungerutil"
)

const (
	flakeNagNotifyName = "FLAKE-PING"
	// defaultTimePeriod is priority/P1 (to get a human to prioritize)
	defaultTimePeriod = 4 * 24 * time.Hour
)

var (
	pinger = c.NewPinger(flakeNagNotifyName).
		SetDescription("This flaky-test issue would love to have more attention.")
	// Only include priorities that you care about. Others won't be pinged
	timePeriods = map[string]time.Duration{
		"priority/P0": 2 * 24 * time.Hour,
		"priority/P1": 8 * 24 * time.Hour,
		"priority/P2": time.Duration(1<<63 - 1),
		"priority/P3": time.Duration(1<<63 - 1),
	}
)

// NagFlakeIssues pings assignees on flaky-test issues
type NagFlakeIssues struct{}

var _ Munger = &NagFlakeIssues{}

func init() {
	n := &NagFlakeIssues{}
	RegisterMungerOrDie(n)
	RegisterStaleComments(n)
}

// Name is the name usable in --pr-mungers
func (NagFlakeIssues) Name() string { return "nag-flake-issues" }

// RequiredFeatures is a slice of 'features' that must be provided
func (NagFlakeIssues) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (NagFlakeIssues) Initialize(config *mgh.Config, features *features.Features) error {
	return nil
}

// EachLoop is called at the start of every munge loop
func (NagFlakeIssues) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (NagFlakeIssues) AddFlags(cmd *cobra.Command, config *mgh.Config) {
}

// findTimePeriod returns how often we should ping based on priority
func findTimePeriod(labels []github.Label) time.Duration {
	priorities := mgh.GetLabelsWithPrefix(labels, "priority/")
	if len(priorities) == 0 {
		return defaultTimePeriod
	}
	// If we have multiple priority labels (shouldn't happen), use the first one
	period, ok := timePeriods[priorities[0]]
	if !ok {
		return defaultTimePeriod
	}
	return period
}

// Munge is the workhorse the will actually make updates to the PR
func (NagFlakeIssues) Munge(obj *mgh.MungeObject) {
	if obj.IsPR() || !obj.HasLabel("kind/flake") {
		return
	}

	comments, ok := obj.ListComments()
	if !ok {
		return
	}

	// Use the pinger to notify assignees:
	// - Set time period based on configuration (at the top of this file)
	// - Mention list of assignees as an argument
	// - Start the ping timer after the last HumanActor comment

	// How often should we ping
	period := findTimePeriod(obj.Issue.Labels)

	// Who are we pinging
	who := mungerutil.GetIssueUsers(obj.Issue).Assignees.Mention().Join()
	if who == "" {
		return
	}

	// When does the pinger start
	startDate := c.LastComment(comments, c.HumanActor(), obj.Issue.CreatedAt)

	// Get a notification if it's time to ping.
	notif := pinger.SetTimePeriod(period).PingNotification(
		comments,
		who,
		startDate,
	)
	if notif != nil {
		obj.WriteComment(notif.String())
	}
}

// StaleComments returns a slice of stale comments
func (NagFlakeIssues) StaleComments(obj *mgh.MungeObject, comments []*github.IssueComment) []*github.IssueComment {
	// Remove all pings written before the last human actor comment
	return c.FilterComments(comments, c.And([]c.Matcher{
		c.MungerNotificationName(flakeNagNotifyName),
		c.CreatedBefore(*c.LastComment(comments, c.HumanActor(), &time.Time{})),
	}))
}
