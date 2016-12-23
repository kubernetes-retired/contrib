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

package features

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/contrib/mungegithub/github"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/yaml"

	parseYaml "github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	ownerFilename = "OWNERS" // file which contains approvers and reviewers
	// RepoFeatureName is how mungers should indicate this is required
	RepoFeatureName = "gitrepos"
)

type assignmentConfig struct {
	Assignees []string `json:assignees yaml:assignees`
	Approvers []string `json:approvers yaml:approvers`
	Reviewers []string `json:reviewers yaml:reviewers`
}

// RepoInfo provides information about users in OWNERS files in a git repo
type RepoInfo struct {
	BaseDir      string
	EnableMdYaml bool
	UseReviewers bool

	enabled    bool
	projectDir string
	approvers  map[string]sets.String
	reviewers  map[string]sets.String
	config     *github.Config
}

func init() {
	RegisterFeature(&RepoInfo{})
}

// Name is just going to return the name mungers use to request this feature
func (o *RepoInfo) Name() string {
	return RepoFeatureName
}

func (o *RepoInfo) walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		glog.Errorf("%v", err)
		return nil
	}
	filename := filepath.Base(path)
	if info.Mode().IsDir() {
		switch filename {
		case ".git":
			return filepath.SkipDir
		case "_output":
			return filepath.SkipDir
		}
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	c := &assignmentConfig{}

	// '.md' files may contain assignees at the top of the file in a yaml header
	// Flag guarded because this is only enabled in some repos
	if o.EnableMdYaml && filename != ownerFilename && strings.HasSuffix(filename, "md") {
		// Parse the yaml header from the file if it exists and marshal into the config
		if err := decodeAssignmentConfig(path, c); err != nil {
			glog.Errorf("%v", err)
			return err
		}

		// Set assignees for this file using the relative path if they were found
		path, err = filepath.Rel(o.projectDir, path)
		if err != nil {
			glog.Errorf("Unable to find relative path between %q and %q: %v", o.projectDir, path, err)
			return err
		}
		o.approvers[path] = sets.NewString(c.Approvers...)
		o.approvers[path].Insert(c.Assignees...)
		o.reviewers[path] = sets.NewString(c.Reviewers...)
		return nil
	}

	if filename != ownerFilename {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		glog.Errorf("%v", err)
		return nil
	}
	defer file.Close()

	if err := yaml.NewYAMLToJSONDecoder(file).Decode(c); err != nil {
		glog.Errorf("%v", err)
		return nil
	}

	path, err = filepath.Rel(o.projectDir, path)
	if err != nil {
		glog.Errorf("Unable to find relative path between %q and %q: %v", o.projectDir, path, err)
		return err
	}
	path = filepath.Dir(path)
	// Make the root explicitly / so its easy to distinguish. Nothing else is `/` anchored
	if path == "." {
		path = "/"
	}
	o.approvers[path] = sets.NewString(c.Approvers...)
	o.approvers[path].Insert(c.Assignees...)
	o.reviewers[path] = sets.NewString(c.Reviewers...)
	return nil
}

// decodeAssignmentConfig will parse the yaml header if it exists and unmarshal it into an assignmentConfig.
// If no yaml header is found, do nothing
// Returns an error if the file cannot be read or the yaml header is found but cannot be unmarshalled
var mdStructuredHeaderRegex = regexp.MustCompile("^---\n(.|\n)*\n---")

func decodeAssignmentConfig(path string, config *assignmentConfig) error {
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	// Parse the yaml header from the top of the file.  Will return an empty string if regex does not match.
	meta := mdStructuredHeaderRegex.FindString(string(fileBytes))

	// Unmarshal the yaml header into the config
	return parseYaml.Unmarshal([]byte(meta), &config)
}

func (o *RepoInfo) updateRepoUsers() error {
	out, err := o.GitCommand([]string{"pull"})
	if err != nil {
		glog.Errorf("Unable to run git pull:\n%s\n%v", string(out), err)
		return err
	}

	out, err = o.GitCommand([]string{"rev-parse", "HEAD"})
	if err != nil {
		glog.Errorf("Unable get sha of HEAD:\n%s\n%v", string(out), err)
		return err
	}
	sha := out

	o.approvers = map[string]sets.String{}
	o.reviewers = map[string]sets.String{}
	err = filepath.Walk(o.projectDir, o.walkFunc)
	if err != nil {
		glog.Errorf("Got error %v", err)
	}
	glog.Infof("Loaded config from %s:%s", o.projectDir, sha)
	glog.V(5).Infof("approvers: %v", o.approvers)
	glog.V(5).Infof("reviewers: %v", o.reviewers)
	return nil
}

// Initialize will initialize the munger
func (o *RepoInfo) Initialize(config *github.Config) error {
	o.enabled = true
	o.config = config
	o.projectDir = path.Join(o.BaseDir, o.config.Project)

	if len(o.BaseDir) == 0 {
		glog.Fatalf("--repo-dir is required with selected munger(s)")
	}
	finfo, err := os.Stat(o.BaseDir)
	if err != nil {
		return fmt.Errorf("Unable to stat --repo-dir: %v", err)
	}
	if !finfo.IsDir() {
		return fmt.Errorf("--repo-dir is not a directory")
	}

	// check if the cloned dir already exists, if yes, cleanup.
	if _, err := os.Stat(o.projectDir); !os.IsNotExist(err) {
		if err := o.cleanUp(o.projectDir); err != nil {
			return fmt.Errorf("Unable to remove old clone directory at %v: %v", o.projectDir, err)
		}
	}

	if cloneUrl, err := o.cloneRepo(); err != nil {
		return fmt.Errorf("Unable to clone %v: %v", cloneUrl, err)
	}
	return o.updateRepoUsers()
}

func (o *RepoInfo) cleanUp(path string) error {
	return os.RemoveAll(path)
}

func (o *RepoInfo) cloneRepo() (string, error) {
	cloneUrl := fmt.Sprintf("https://github.com/%s/%s.git", o.config.Org, o.config.Project)
	output, err := o.gitCommandDir([]string{"clone", cloneUrl, o.projectDir}, o.BaseDir)
	if err != nil {
		glog.Errorf("Failed to clone github repo: %s", output)
	}
	return cloneUrl, err
}

// EachLoop is called at the start of every munge loop
func (o *RepoInfo) EachLoop() error {
	if !o.enabled {
		return nil
	}
	_, err := o.GitCommand([]string{"remote", "update"})
	if err != nil {
		glog.Errorf("Unable to git remote update: %v", err)
	}
	return o.updateRepoUsers()
}

// AddFlags will add any request flags to the cobra `cmd`
func (o *RepoInfo) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.BaseDir, "repo-dir", "", "Path to perform checkout of repository")
	cmd.Flags().BoolVar(&o.EnableMdYaml, "enable-md-yaml", false, "If true, look for assignees in md yaml headers.")
	cmd.Flags().BoolVar(&o.UseReviewers, "use-reviewers", false, "Use \"reviewers\" rather than \"approvers\" for review")
}

// GitCommand will execute the git command with the `args` within the project directory.
func (o *RepoInfo) GitCommand(args []string) ([]byte, error) {
	return o.gitCommandDir(args, o.projectDir)
}

// GitCommandDir will execute the git command with the `args` within the 'dir' directory.
func (o *RepoInfo) gitCommandDir(args []string, cmdDir string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cmdDir
	return cmd.CombinedOutput()
}

// FindOwnersForPath returns the OWNERS file further down the tree for a file
func (o *RepoInfo) FindOwnersForPath(path string) string {
	d := path

	for {
		// special case the root
		if d == "." || d == "" {
			d = "/"
		}
		_, ok := o.approvers[d]
		if ok {
			return d
		}
		if d == "/" {
			break
		}
		d = filepath.Dir(d)
		d = strings.TrimSuffix(d, "/")
	}
	return ""
}

// peopleForPath returns a set of users who are assignees to the
// requested file. The path variable should be a full path to a filename
// and not directory as the final directory will be discounted if enableMdYaml is true
// leafOnly indicates whether only the OWNERS deepest in the tree (closest to the file)
// should be returned or if all OWNERS in filepath should be returned
func peopleForPath(path string, people map[string]sets.String, leafOnly bool, enableMdYaml bool) sets.String {
	d := path
	if !enableMdYaml {
		// if path is a directory, this will remove the leaf directory
		d = filepath.Dir(path)
	}

	out := sets.NewString()
	for {
		// special case the root
		if d == "" {
			d = "/"
		}
		s, ok := people[d]
		if ok {
			out = out.Union(s)
			if leafOnly && out.Len() > 0 {
				break
			}
		}
		if d == "/" {
			break
		}
		d, _ = filepath.Split(d)
		d = strings.TrimSuffix(d, "/")
	}
	return out
}

// LeafApprovers returns a set of users who are the closest approvers to the
// requested file. If pkg/OWNERS has user1 and pkg/util/OWNERS has user2 this
// will only return user2 for the path pkg/util/sets/file.go
func (o *RepoInfo) LeafApprovers(path string) sets.String {
	return peopleForPath(path, o.approvers, true, o.EnableMdYaml)
}

// Approvers returns ALL of the users who are approvers for the
// requested file (including approvers in parent dirs' OWNERS).
// If pkg/OWNERS has user1 and pkg/util/OWNERS has user2 this
// will return both user1 and user2 for the path pkg/util/sets/file.go
func (o *RepoInfo) Approvers(path string) sets.String {
	return peopleForPath(path, o.approvers, false, o.EnableMdYaml)
}

// LeafReviewers returns a set of users who are the closest reviewers to the
// requested file. If pkg/OWNERS has user1 and pkg/util/OWNERS has user2 this
// will only return user2 for the path pkg/util/sets/file.go
func (o *RepoInfo) LeafReviewers(path string) sets.String {
	if !o.UseReviewers {
		return o.LeafApprovers(path)
	}
	return peopleForPath(path, o.reviewers, true, o.EnableMdYaml)
}

// Reviewers returns ALL of the users who are reviewers for the
// requested file (including reviewers in parent dirs' OWNERS).
// If pkg/OWNERS has user1 and pkg/util/OWNERS has user2 this
// will return both user1 and user2 for the path pkg/util/sets/file.go
func (o *RepoInfo) Reviewers(path string) sets.String {
	if !o.UseReviewers {
		return o.Approvers(path)
	}
	return peopleForPath(path, o.reviewers, false, o.EnableMdYaml)
}
