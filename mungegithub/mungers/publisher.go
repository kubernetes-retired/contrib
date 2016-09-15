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
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/kubernetes/pkg/util/errors"
)

type info struct {
	repo   string
	branch string
	dir    string
}

// a publishing target
type target struct {
	dstRepo string
	// multiple sources mapping to different directories in the dstRepo
	srcToDst map[info]string
}

// PublisherMunger publishes content from one repository to another one.
type PublisherMunger struct {
	// Command for the 'publisher' munger to run periodically.
	PublishCommand string
	// base to all repos
	baseDir string
	// location to write the netrc file needed for github authentication
	netrcDir     string
	targets      []target
	features     *features.Features
	githubConfig *github.Config
}

func init() {
	publisherMunger := &PublisherMunger{}
	RegisterMungerOrDie(publisherMunger)
}

// Name is the name usable in --pr-mungers
func (p *PublisherMunger) Name() string { return "publisher" }

// RequiredFeatures is a slice of 'features' that must be provided
func (p *PublisherMunger) RequiredFeatures() []string { return []string{features.RepoFeatureName} }

// Initialize will initialize the munger
func (p *PublisherMunger) Initialize(config *github.Config, features *features.Features) error {
	p.baseDir = features.Repos.BaseDir
	if len(p.baseDir) == 0 {
		glog.Fatalf("--repo-dir is required with selected munger(s)")
	}
	clientGo := target{
		dstRepo: "client-go",
		srcToDst: map[info]string{
			info{repo: config.Project, branch: "release-1.4", dir: "staging/src/k8s.io/client-go/1.4"}: "1.4",
			// TODO: uncomment this when 1.5 folder is created
			info{repo: config.Project, branch: "master", dir: "staging/src/k8s.io/client-go/1.5"}: "1.5",
		},
	}
	p.targets = []target{clientGo}
	glog.Infof("pulisher munger map: %#v\n", p.targets)
	p.features = features
	p.githubConfig = config
	return nil
}

// git clone dstURL to dst
func clone(dst string, dstURL string) error {
	err := exec.Command("rm", "-rf", dst).Run()
	if err != nil {
		return err
	}
	err = exec.Command("mkdir", "-p", dst).Run()
	if err != nil {
		return err
	}
	err = exec.Command("git", "clone", dstURL, dst).Run()
	if err != nil {
		return err
	}
	return nil
}

// construct checks out the source repo, copy the contents to the destination,
// returns a commit message snippet and error.
func construct(src, srcURL, srcBranch, dst, dstRelative string) (string, error) {
	err := exec.Command("pushd", src).Run()
	if err != nil {
		return "", err
	}
	err := exec.Command("git", "checkout", srcBranch).Run()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "checkout", srcBranch).CombinedOutput()
	if err != nil {
		return "", err
	}
	commit_hash := string(out)
	err := exec.Command("popd").Run()
	if err != nil {
		return "", err
	}
	err := exec.Command("pushd", dst).Run()
	if err != nil {
		return "", err
	}
	err := exec.Command("rm", "-rf", dstRelative).Run()
	if err != nil {
		return "", err
	}
	err := exec.Command("cp", "-a", src+"/.", dst).Run()
	if err != nil {
		return "", err
	}
	// rename _vendor to vendor
	err := exec.Command("find", dstRelative, "-depth", "-name", "_vendor", "-type", "d", "-execdir", "mv", "{}", "vendor", "\\").Run()
	if err != nil {
		return "", err
	}
	commitMessage := fmt.Sprintf("Directory %s is copied from\n", dstRelative)
	commitMessage += fmt.Sprintf("%s, branch %s,\n", srcURL, srcBranch)
	commitMessage += fmt.Sprintf("last commit is %s\n", commit_hash)
	return commitMessage, nil
}

// EachLoop is called at the start of every munge loop
func (p *PublisherMunger) EachLoop() error {
	var errlist []error
Target:
	for _, target := range p.targets {
		// clone the destination repo
		dst := filepath.Join(p.baseDir, target.dstRepo, "")
		dstURL := fmt.Sprintf("https://github.com/%s/%s.git", p.githubConfig.Org, target.dstRepo)
		err := clone(dst, dstURL)
		if err != nil {
			glog.Errorf("Failed to clone %s.\nOutput: %s\nError: %s", dstURL, output, err)
			errlist = append(errlist, err)
			continue Target
		} else {
			glog.Infof("Successfully clone %s", dstURL)
		}
		// construct the destination directory
		var commitMessage string
		for srcInfo, dstDir := range target.srcToDst {
			src := filepath.Join(p.baseDir, srcInfo.repo, srcInfo.dir)
			dst := filepath.Join(p.baseDir, target.dstRepo, dstDir)
			srcURL := fmt.Sprintf("https://github.com/%s/%s.git", p.githubConfig.Org, srcInfo.repo)

			snippet, err := construct(src, srcURL, srcInfo.branch, dst, dstDir)
			if err != nil {
				glog.Errorf("Failed to construct %s.\nOutput: %s\nError: %s", dst, output, err)
				errlist = append(errlist, err)
				continue Target
			} else {
				commitMessage += snippet
				glog.Infof("Successfully construct %s: %s", dst, output)
			}
		}

		// publish the destination directory
		cmd = exec.Command("./publish.sh", dst, p.githubConfig.Token(), p.netrcDir, strings.TrimSpace(commitMessage))
		output, err = cmd.CombinedOutput()
		if err != nil {
			glog.Errorf("Failed to publish %s.\nOutput: %s\nError: %s", dst, output, err)
			errlist = append(errlist, err)
			continue Target
		} else {
			glog.Infof("Successfully publish %s: %s", dst, output)
		}
	}
	return errors.NewAggregate(errlist)
}

// AddFlags will add any request flags to the cobra `cmd`
func (p *PublisherMunger) AddFlags(cmd *cobra.Command, config *github.Config) {
	cmd.Flags().StringVar(&p.netrcDir, "netrc-dir", "", "Location to write the netrc file needed for github authentication.")
}

// Munge is the workhorse the will actually make updates to the PR
func (p *PublisherMunger) Munge(obj *github.MungeObject) {}
