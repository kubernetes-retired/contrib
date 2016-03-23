/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Action is an interface for actions that should be taken by the Checker if Trigger returns true.
type Action interface {
	Do() error
}

// PrintAction is a simple Action implementation that stores a string and logs it during the Do() call.
type PrintAction struct {
	ToPrint string
}

// Do logs a ToPrint string.
func (a *PrintAction) Do() error {
	glog.Infof(a.ToPrint)
	return nil
}

const (
	scalabilityHighFrequencyCron = "@hourly"
	scalabilityLowFrequencyCron  = "H H/12 * * *"
	gceScalabilityTestConfigPath = "hack/jenkins/job-configs/kubernetes-jenkins/kubernetes-e2e.yaml"
)

// return true, new config contents if changes are needed, or false, "" if no changes are made
func substituteCronStringAfter(repoPath string, testConfigPath string, delim string, targetCron string) (bool, string) {
	file, err := os.Open(repoPath + "/" + testConfigPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	currentConfig := string(data)
	if err != nil {
		panic(err)
	}
	index := strings.Index(currentConfig, delim)
	cronIndex := strings.Index(currentConfig[index:], "cron-string")
	cronIndex += index
	newConfig := currentConfig[0:cronIndex]
	cronEndlineIndex := strings.Index(currentConfig[cronIndex:], "\n")
	cronEndlineIndex += cronIndex
	if currentConfig[cronIndex:cronEndlineIndex] == fmt.Sprintf("cron-string: '%v'", targetCron) {
		return false, ""
	}
	newConfig = newConfig + fmt.Sprintf("cron-string: '%v'", targetCron)
	newConfig = newConfig + currentConfig[cronEndlineIndex:]
	return true, newConfig
}

func applyChangesToScalabilityConfig(repoPath string, targetCron string) bool {
	changed, newConfig := substituteCronStringAfter(repoPath, gceScalabilityTestConfigPath, "gce-scalability", targetCron)
	if !changed {
		return false
	}
	ioutil.WriteFile(repoPath+"/"+gceScalabilityTestConfigPath, []byte(newConfig), 0644)
	return true
}

func pushChanges(repoPath string, gitHubUser string, gitHubToken string, additionalLog string, branchName string) error {
	command := exec.Command("./pr-generator.sh", repoPath, gitHubUser, gitHubToken, "Automatic test reconfiguration "+additionalLog, branchName)
	// Pipe script output to stdout to make it debugable - I'd love to be able to pipe it to glog...
	stdoutWriter := bufio.NewWriter(os.Stdout)
	command.Stdout = stdoutWriter
	errBuffer := &bytes.Buffer{}
	command.Stderr = errBuffer
	err := command.Run()
	stdoutWriter.Flush()
	if err != nil {
		glog.Errorf("Command err: %v", err)
		glog.Errorf("StdErr from command: %v", errBuffer.String())
	}
	return err
}

func createPR(repoPath string, gitHubUser string, gitHubToken string, branchName string, timestamp time.Time) error {
	var transport http.RoundTripper
	transport = &http.Transport{}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gitHubToken})
	transport = &oauth2.Transport{
		Base:   transport,
		Source: oauth2.ReuseTokenSource(nil, ts),
	}
	client := &http.Client{
		Transport: transport,
	}
	gitHubClient := github.NewClient(client)

	title := fmt.Sprintf("Test adjuster change on %v", timestamp)
	head := gitHubUser + ":" + branchName
	base := "master"
	body := "Automatic scalability test reconfiguration." // + " cc @k8s-oncall"

	pull := &github.NewPullRequest{
		Title: &title,
		Head:  &head,
		Base:  &base,
		Body:  &body,
	}
	_, response, err := gitHubClient.PullRequests.Create(gitHubUser, "kubernetes", pull)
	if err != nil {
		glog.Errorf("error: %v\nresponse: %v\n", err, *response)
	}
	return err
}

// IncreaseScalability100FrequecyAction is an action that creates a PR that increases a frequency of scalability-gce (100) suite.
type IncreaseScalability100FrequecyAction struct {
	repo *GitRepo
}

// NewIncreaseScalability100FrequecyAction is a constructor for IncreaseScalability100FrequecyAction
func NewIncreaseScalability100FrequecyAction(repo *GitRepo) *IncreaseScalability100FrequecyAction {
	action := &IncreaseScalability100FrequecyAction{
		repo: repo,
	}
	return action
}

// Do modifies the repo by increasing the frequency of scalability-gce (100) suite and creating a PR for this change.
func (a *IncreaseScalability100FrequecyAction) Do() error {
	repoPath := a.repo.LockAndGetRepo()
	defer a.repo.UnlockRepo()
	changed := applyChangesToScalabilityConfig(repoPath, scalabilityHighFrequencyCron)
	if !changed {
		return nil
	}
	now := time.Now()
	branchName := fmt.Sprintf("test-adjuster-%v%v%v-%v%v%v", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())

	err := pushChanges(repoPath, a.repo.user, a.repo.token, "increasing scalability 100 frequency", branchName)
	if err != nil {
		return err
	}
	err = createPR(repoPath, a.repo.user, a.repo.token, branchName, now)
	return err
}

// DecreaseScalability100FrequecyAction is an action that creates a PR that decreases a frequency of scalability-gce (100) suite.
type DecreaseScalability100FrequecyAction struct {
	repo *GitRepo
}

// NewDecreaseScalability100FrequecyAction is a constructor for DecreaseScalability100FrequecyAction
func NewDecreaseScalability100FrequecyAction(repo *GitRepo) *DecreaseScalability100FrequecyAction {
	action := &DecreaseScalability100FrequecyAction{
		repo: repo,
	}
	return action
}

// Do modifies the repo by decreasing the frequency of scalability-gce (100) suite and creating a PR for this change.
func (a *DecreaseScalability100FrequecyAction) Do() error {
	repoPath := a.repo.LockAndGetRepo()
	defer a.repo.UnlockRepo()
	changed := applyChangesToScalabilityConfig(repoPath, scalabilityLowFrequencyCron)
	if !changed {
		return nil
	}
	now := time.Now()
	branchName := fmt.Sprintf("test-adjuster-%v%v%v-%v%v%v", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())

	err := pushChanges(repoPath, a.repo.user, a.repo.token, "decreasing scalability 100 frequency", branchName)
	if err != nil {
		return err
	}
	err = createPR(repoPath, a.repo.user, a.repo.token, branchName, now)
	return err
}
