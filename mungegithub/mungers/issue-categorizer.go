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

package mungers

import (
	"net/http"
	"net/url"
	"strings"

	"k8s.io/contrib/mungegithub/github"
	"k8s.io/contrib/mungegithub/mungers/matchers/event"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/features"
)

// LabelMunger will update a label on a PR based on how many lines are changed.
// It will exclude certain files in it's calculations based on the config
// file provided in --generated-files-config
type LabelMunger struct {
}

// Initialize will initialize the munger
func (LabelMunger) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// Name is the name usable in --pr-mungers
func (LabelMunger) Name() string { return "issue-triager" }

// RequiredFeatures is a slice of 'features' that must be provided
func (LabelMunger) RequiredFeatures() []string { return []string{} }

// AddFlags will add any request flags to the cobra `cmd`
func (LabelMunger) AddFlags(cmd *cobra.Command, config *github.Config) {

}

func init() {
	s := &LabelMunger{}
	RegisterMungerOrDie(s)
}

// EachLoop is called at the start of every munge loop
func (LabelMunger) EachLoop() error { return nil }

// Munge is the workhorse the will actually make updates to the PR
func (s *LabelMunger) Munge(obj *github.MungeObject) {
	//this munger only works on issues
	if obj.IsPR() {
		return
	}

	issue := obj.Issue
	if obj.HasLabel("kind/flake") {
		return
	}

	tLabels := github.GetLabelsWithPrefix(issue.Labels, "team/")
	cLabels := github.GetLabelsWithPrefix(issue.Labels, "component/")

	if len(tLabels) != 0 || len(cLabels) != 0 {
		updateModel(obj)
		return
	}

	routingLabelsToApply, err := http.PostForm("http://issue-triager-service:5000",
		url.Values{"title": {*issue.Title}, "body": {*issue.Body}})

	if err != nil {
		//handle the error
		glog.Error(err)
		return
	}
	defer routingLabelsToApply.Body.Close()
	response, err := ioutil.ReadAll(routingLabelsToApply.Body)
	if routingLabelsToApply.StatusCode != 200 {
		glog.Errorf("%d: %s", routingLabelsToApply.StatusCode, response)
		return
	}

	obj.AddLabels(strings.Split(string(response), ","))
}

func getHumanCorrectedLabel(obj *github.MungeObject, s string) *string {
	myEvents, _ := obj.GetEvents()

	botEvents := event.FilterEvents(myEvents, event.And([]event.Matcher{event.BotActor(), event.AddLabel{}, event.LabelPrefix(s)}))

	if botEvents.Empty() {
		glog.Infof("Found no bot %s labeling for issue %d ", obj.Issue.Number, s)
		return nil
	}

	humanEventsAfter := event.FilterEvents(
		myEvents,
		event.And([]event.Matcher{
			event.HumanActor(),
			event.AddLabel{},
			event.LabelPrefix(s),
			event.CreatedAfter(*botEvents.GetLast().CreatedAt),
		}),
	)

	if humanEventsAfter.Empty() {
		glog.Infof("Found no human corrections of %s label for issue %d", obj.Issue.Number, s)
		return nil
	}
	lastHumanLabel := humanEventsAfter.GetLast()

	glog.Infof("Recopying human-added label: %s for PR %d", *lastHumanLabel.Label.Name, *obj.Issue.Number)
	obj.RemoveLabel(*lastHumanLabel.Label.Name)
	obj.AddLabel(*lastHumanLabel.Label.Name)
	return lastHumanLabel.Label.Name
}

func updateModel(obj *github.MungeObject) {
	newLabels := []string{}

	newTeamLabel := getHumanCorrectedLabel(obj, "team")
	if newTeamLabel != nil {
		newLabels = append(newLabels, *newTeamLabel)
	}

	newComponentLabel := getHumanCorrectedLabel(obj, "component")
	if newComponentLabel != nil {
		newLabels = append(newLabels, *newComponentLabel)
	}

	if len(newLabels) != 0 {
		glog.Infof("Updating the models on the server")
		_, err := http.PostForm("http://issue-triager-service:5000",
			url.Values{"titles": []string {*obj.Issue.Title},
				"bodies": []string {*obj.Issue.Body},
				"labels": newLabels})
		if err != nil{
			glog.Error(err)
		}
	}
}