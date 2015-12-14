/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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
	"bytes"
	goflag "flag"
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/contrib/mungegithub/github"
)

var (
	base        string
	last        int
	lastTime    time.Time
	current     int
	currentTime time.Time
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&last, "last-release-pr", 0, "The PR number of the last versioned release.")
	cmd.Flags().IntVar(&current, "current-release-pr", 0, "The PR number of the current versioned release.")
	cmd.Flags().StringVar(&base, "base", "master", "The base branch name for PRs to look for.")
	cmd.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
}

func mergedAt(num int, config *github.Config) time.Time {
	obj, err := config.GetObject(num)
	if err != nil {
		glog.Fatalf("Unable to get munge object for %d: %v", num, err)
	}
	t := obj.MergedAt()
	if t == nil {
		glog.Fatalf("Unable to get merge time for %d: %v", num, err)
	}
	return *t
}

func main() {
	config := &github.Config{}
	cmd := &cobra.Command{
		Use: "release-notes --last-release-pr=NUM --current-release-pr=NUM",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := config.PreExecute(); err != nil {
				return err
			}
			if last == 0 || current == 0 {
				return fmt.Errorf("Must set (at least) --last-releae-pr and --current-release-pr")
			}
			lastTime = mergedAt(last, config)
			currentTime = mergedAt(current, config)

			buffer := &bytes.Buffer{}
			config.ForEachIssueDo("closed", []string{"release-note"}, func(obj *github.MungeObject) error {
				if !obj.IsForBranch(base) {
					return nil
				}

				mergedAt := obj.MergedAt()
				if mergedAt.Before(lastTime) {
					return nil
				}
				if mergedAt.After(currentTime) {
					return nil
				}
				fmt.Fprintf(buffer, "   * %s (#%d, @%s)\n", *obj.Issue.Title, *obj.Issue.Number, *obj.Issue.User.Login)
			})
			fmt.Println()
			fmt.Printf("Release notes for PRs between #%d and #%d against branch %q:\n\n", last, current, base)
			fmt.Printf("%s", buffer.Bytes())
			return nil
		},
	}
	addFlags(cmd)
	config.AddRootFlags(cmd)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
