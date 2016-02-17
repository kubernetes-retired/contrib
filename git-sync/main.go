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

// git-sync is a command that pull a git repository to a local directory.

package main // import "k8s.io/contrib/git-sync"

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

var flRepo = flag.String("repo", envString("GIT_SYNC_REPO", ""), "git repo url")
var flBranch = flag.String("branch", envString("GIT_SYNC_BRANCH", "master"), "git branch")
var flRev = flag.String("rev", envString("GIT_SYNC_REV", "HEAD"), "git rev")
var flDest = flag.String("dest", envString("GIT_SYNC_DEST", ""), "destination path")
var flWait = flag.Int("wait", envInt("GIT_SYNC_WAIT", 0), "number of seconds to wait before next sync")
var flOneTime = flag.Bool("one-time", envBool("GIT_SYNC_ONE_TIME", false), "exit after the initial checkout")
var flDepth = flag.Int("depth", envInt("GIT_SYNC_DEPTH", 0), "shallow clone with a history truncated to the specified number of commits")

var flUsername = flag.String("username", envString("GIT_SYNC_USERNAME", ""), "username")
var flPassword = flag.String("password", envString("GIT_SYNC_PASSWORD", ""), "password")

func envString(key, def string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return def
}

func envBool(key string, def bool) bool {
	if env := os.Getenv(key); env != "" {
		res, err := strconv.ParseBool(env)
		if err != nil {
			return def
		}

		return res
	}
	return def
}

func envInt(key string, def int) int {
	if env := os.Getenv(key); env != "" {
		val, err := strconv.Atoi(env)
		if err != nil {
			log.Printf("invalid value for %q: using default: %q", key, def)
			return def
		}
		return val
	}
	return def
}

const usage = "usage: GIT_SYNC_REPO= GIT_SYNC_DEST= [GIT_SYNC_BRANCH= GIT_SYNC_WAIT= GIT_SYNC_DEPTH= GIT_SYNC_USERNAME= GIT_SYNC_PASSWORD= GIT_SYNC_ONE_TIME=] git-sync -repo GIT_REPO_URL -dest PATH [-branch -wait -username -password -depth -one-time]"

func main() {
	flag.Parse()
	if *flRepo == "" || *flDest == "" {
		flag.Usage()
		log.Fatal(usage)
	}
	if _, err := exec.LookPath("git"); err != nil {
		log.Fatalf("required git executable not found: %v", err)
	}

	if *flUsername != "" && *flPassword != "" {
		log.Println("creating .netrc file (required for https authentication)")
		if err := createNetrcFile(*flUsername, *flPassword, *flRepo); err != nil {
			log.Fatalf("error creating .netrc file: %v", err)
		}
	}

	if *flOneTime {
		if err := syncRepo(*flRepo, *flDest, *flBranch, *flRev, *flDepth); err != nil {
			log.Fatalf("error syncing repo: %v", err)
		}

		os.Exit(0)
	}

	for {
		if err := syncRepo(*flRepo, *flDest, *flBranch, *flRev, *flDepth); err != nil {
			log.Fatalf("error syncing repo: %v", err)
		}
		log.Printf("wait %d seconds", *flWait)
		time.Sleep(time.Duration(*flWait) * time.Second)
		log.Println("done")
	}
}

// syncRepo syncs the branch of a given repository to the destination at the given rev.
func syncRepo(repo, dest, branch, rev string, depth int) error {
	gitRepoPath := path.Join(dest, ".git")
	_, err := os.Stat(gitRepoPath)
	switch {
	case os.IsNotExist(err):
		// clone repo
		args := []string{"clone", "--no-checkout", "-b", branch}
		if depth != 0 {
			args = append(args, "-depth")
			args = append(args, string(depth))
		}
		args = append(args, repo)
		args = append(args, dest)
		cmd := exec.Command("git", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error cloning repo %q: %v: %s", strings.Join(cmd.Args, " "), err, string(output))
		}
		log.Printf("clone %q: %s", repo, string(output))
	case err != nil:
		return fmt.Errorf("error checking if repo exist %q: %v", gitRepoPath, err)
	}

	// fetch branch
	cmd := exec.Command("git", "fetch", "origin", branch)
	cmd.Dir = dest
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running command %q: %v: %s", strings.Join(cmd.Args, " "), err, string(output))
	}
	log.Printf("fetch %q: %s", branch, string(output))

	// reset working copy
	cmd = exec.Command("git", "reset", "--hard", rev)
	cmd.Dir = dest
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running command %q : %v: %s", strings.Join(cmd.Args, " "), err, string(output))
	}
	log.Printf("reset %q: %v", rev, string(output))

	// set file permissions
	cmd = exec.Command("chmod", "-R", "744", dest)
	cmd.Dir = dest
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running command %q : %v: %s", strings.Join(cmd.Args, " "), err, string(output))
	}

	return nil
}

// https://www.kernel.org/pub/software/scm/git/docs/howto/setup-git-server-over-http.txt
// Step 3: setup the client
func createNetrcFile(username, password, gitURL string) error {
	home := os.Getenv("HOME")
	netrc, err := os.Create(fmt.Sprintf("%v/.netrc", home))
	if err != nil {
		return err
	}
	defer netrc.Close()

	url, err := url.Parse(gitURL)
	if err != nil {
		return err
	}

	netrc.WriteString(fmt.Sprintf("machine %v\n", url.Host))
	netrc.WriteString(fmt.Sprintf("login %v\n", username))
	netrc.WriteString(fmt.Sprintf("password %v\n", password))
	return nil
}
