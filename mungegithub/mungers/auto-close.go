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
	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	c "k8s.io/contrib/mungegithub/mungers/matchers/comment"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/yaml"

	"bytes"
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

const (
	closeNotification = "ClosingNotifier"
)

// AutoCloseMunger will automatically close Issues and PRs that do not have the required
// labels that can be set through the templates
type AutoCloseMunger struct {
	Path               string
	requiredIssueLabel sets.String
	requiredPRLabel    sets.String
}

func init() {
	aC := &AutoCloseMunger{}
	RegisterMungerOrDie(aC)
}

// Name is the name usable in --pr-mungers
func (AutoCloseMunger) Name() string { return "auto-close" }

type configRequiredPrefix struct {
	IssueRequiredLabels []string `json:"issueRequiredLabels,omitempty" yaml:"issueRequiredLabels,omitempty"`
	PrRequiredLabels    []string `json:"prRequiredLabels,omitempty" yaml:"prRequiredLabels,omitempty"`
}

// RequiredFeatures is a slice of 'features' that must be provided
func (AutoCloseMunger) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (aC *AutoCloseMunger) Initialize(config *github.Config, features *features.Features) error {
	if len(aC.Path) == 0 {
		glog.Fatalf("--required-label-prefixes is required with the block-path munger")
	}
	file, err := os.Open(aC.Path)
	if err != nil {
		glog.Fatalf("Failed to load required-label-prefixes config: %v", err)
	}
	defer file.Close()

	c := &configRequiredPrefix{}
	if err := yaml.NewYAMLToJSONDecoder(file).Decode(c); err != nil {
		glog.Fatalf("Failed to decode the required-label-prefixes config: %v", err)
	}

	aC.requiredIssueLabel = sets.NewString(c.IssueRequiredLabels...)
	aC.requiredPRLabel = sets.NewString(c.PrRequiredLabels...)
	return nil
}

// EachLoop is called at the start of every munge loop
func (AutoCloseMunger) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
// AddFlags will add any request flags to the cobra `cmd`
func (aC *AutoCloseMunger) AddFlags(cmd *cobra.Command, config *github.Config) {
	cmd.Flags().StringVar(&aC.Path, "required-label-prefixes", "", "file containing prefixes of labels required")
}

// Munge is the workhorse the will actually make updates to the PR
func (aC *AutoCloseMunger) Munge(obj *github.MungeObject) {
	labels := obj.LabelSet()
	if !obj.IsPR() {
		//it's an issue
		missingSet := hasRequiredLabels(labels, aC.requiredIssueLabel)
		if missingSet.Len() != 0 {
			if !alreadyNotified(obj) {
				createIssueNotification(obj, missingSet)
			}
			pr, _ := obj.GetPR()
			if *pr.State != "closed" {
				obj.ClosePR()
			}
		}
	} else {
		missingSet := hasRequiredLabels(labels, aC.requiredPRLabel)
		if missingSet.Len() != 0 {
			if !alreadyNotified(obj) {
				createPRNotification(obj, missingSet)
			}
			pr, _ := obj.GetPR()
			if *pr.State != "closed" {
				obj.ClosePR()
			}
		}
	}

}

func hasRequiredLabels(actualLabels, requiredLabels sets.String) sets.String {
	var hasLabel bool
	missingSet := sets.NewString()
	for requiredPrefix := range requiredLabels {
		hasLabel = false
		for label := range actualLabels {
			if strings.HasPrefix(label, requiredPrefix) {
				hasLabel = true
			}

		}
		if !hasLabel {
			missingSet.Insert(requiredPrefix)
		}
	}
	return missingSet
}

func createIssueNotification(obj *github.MungeObject, missingSet sets.String) {
	context := bytes.NewBufferString("")
	context.WriteString("This Issue Is Missing the Following Required Label Types\n")
	for lab := range missingSet {
		context.WriteString(fmt.Sprintf("- %s\n", lab))
	}
	c.Notification{closeNotification, "Issue Missing Required Labels", context.String()}.Post(obj)
}

func createPRNotification(obj *github.MungeObject, missingSet sets.String) {
	context := bytes.NewBufferString("")
	context.WriteString("This PR Is Missing the Following Required Label Types\n")
	for lab := range missingSet {
		context.WriteString(fmt.Sprintf("- %s\n", lab))
	}
	c.Notification{closeNotification, "PR Missing Required Labels", context.String()}.Post(obj)
}

func alreadyNotified(obj *github.MungeObject) bool {

	notificationMatcher := c.MungerNotificationName(closeNotification)
	comments, ok := obj.ListComments()
	if !ok {
		fmt.Errorf("Unable to ListComments for %d", obj.Number())
		return false
	}

	notifications := c.FilterComments(comments, notificationMatcher)
	return !notifications.Empty()
}
