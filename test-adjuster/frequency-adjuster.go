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
	"flag"
	"io/ioutil"
	"strings"
	"sync"
	"time"
)

var (
	repoPath      = flag.String("repo-path", "/kubernetes", "Path to the clone of Kubernetes repository.")
	tokenFile     = flag.String("token-file", "/token", "Path to a file with saved github token.")
	gitHubUser    = flag.String("github-user", "gmarek", "User to use for github authentication.")
	pollFrequency = 10 * time.Minute
	checkers      = []*Checker{}
)

type GitRepo struct {
	user     string
	token    string
	path     string
	repoLock sync.Mutex
}

func (r *GitRepo) LockAndGetRepo() string {
	r.repoLock.Lock()
	return r.path
}

func (r *GitRepo) UnlockRepo() {
	r.repoLock.Unlock()
}

func readToken() string {
	data, err := ioutil.ReadFile(*tokenFile)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(data))
}

func registerChecker(trigger Trigger, action Action) {
	checkers = append(checkers, NewChecker(trigger, action))
}

func main() {
	flag.Parse()
	// repo := GitRepo{user: *gitHubUser, token: readToken(), path: *repoPath}
	// increaseAction := NewIncreaseScalability100FrequecyAction(&repo)
	// decreaseAction := NewDecreaseScalability100FrequecyAction(&repo)
	// registerChecker(NewBasicUnstableTestTrigger("kubernetes-kubemark-500-gce"), increaseAction)
	// registerChecker(NewBasicStableTestTrigger("kubernetes-kubemark-500-gce"), decreaseAction)

	registerChecker(NewBasicUnstableTestTrigger("kubernetes-kubemark-500-gce"), &PrintAction{ToPrint: "Printing: unstable"})
	registerChecker(NewBasicStableTestTrigger("kubernetes-kubemark-500-gce"), &PrintAction{ToPrint: "Printing: stable"})

	for {
		for _, checker := range checkers {
			checker.Run()
		}
		time.Sleep(pollFrequency)
	}
}
