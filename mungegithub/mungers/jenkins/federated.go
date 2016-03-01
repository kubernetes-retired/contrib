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

package jenkins

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/golang/glog"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

// FederatedBuilder is how we talk to the repository of build results
type FederatedBuilder struct {
	bucketName string
	basePath   string

	name string

	ctx    context.Context
	client *storage.Client
	bucket *storage.BucketHandle
}

var _ Builder = &FederatedBuilder{}

func NewFederatedBuilder(name string, path string) (*FederatedBuilder, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %q", path)
	}

	if u.Scheme != "gs" {
		return nil, fmt.Errorf("unhandled scheme in URL %q", path)
	}

	bucketName := u.Host
	basePath := u.Path
	if strings.HasPrefix(basePath, "/") {
		basePath = basePath[1:]
	}
	if basePath != "" && !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx, cloud.WithScopes(storage.ScopeReadOnly))
	if err != nil {
		return nil, fmt.Errorf("error building storage client: %v", err)
	}
	f := &FederatedBuilder{
		ctx:        ctx,
		client:     client,
		bucketName: bucketName,
		bucket:     client.Bucket(bucketName),
		basePath:   basePath,
		name:       name,
	}
	return f, nil
}

func (f *FederatedBuilder) Close() error {
	if f.client != nil {
		err := f.client.Close()
		if err != nil {
			return err
		}
		f.client = nil
	}
	return nil
}

// Read a file from google cloud storage; relativePath is relative to basePath
func (f *FederatedBuilder) readObject(relativePath string) ([]byte, error) {
	path := f.basePath + relativePath
	glog.V(3).Infof("Fetching object %q in bucket %q", path, f.bucketName)
	rc, err := f.bucket.Object(path).NewReader(f.ctx)
	if err != nil {
		return nil, fmt.Errorf("error opening object %q in bucket %q: %v", path, f.bucketName, err)
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

// Relies on a build result file in `<buildID>/result.txt`.  The build result
// should be a text file, comprising lines of `key=value`.  Currently the
// followings keys are recognized (and other keys are ignored):
//
// SUCCESS: 'true' or 'false' depending on whether the job was successful
func (f *FederatedBuilder) readBuildResult(buildID string) (*BuildResult, error) {
	data, err := f.readObject(buildID + "/result.txt")
	if err != nil {
		return nil, err
	}

	humanName := f.name + ":" + buildID

	glog.V(8).Infof("Got data: %s", string(data))

	fields, err := parseDelimited(data)
	if err != nil {
		return nil, fmt.Errorf("error parsing build result for %s: %v", humanName, err)
	}

	success := true
	if fields["SUCCESS"] != "" {
		success, err = strconv.ParseBool(fields["SUCCESS"])
		if err != nil {
			return nil, fmt.Errorf("unexpected value for success in %s: %q", humanName, fields["SUCCESS"])
		}
	}

	url := "https://storage.cloud.google.com/" + f.bucketName + "/" + f.basePath + buildID + "/"
	br := &BuildResult{
		Success: success,
		BuildID: buildID,
		URL:     url,
	}

	return br, nil
}

// GetLastCompletedBuild does just that
//
// Relies on a text file `latest-build.txt` which is the same as the Jenkins
// builder uploads.  It should be a plain text file, comprising lines of
// `key=value`.  Values pertain to the most recent build.
//
// The following keys are recognized (and other keys are ignored):
//
// BUILD_NUMBER: The build identifier for the build
func (f *FederatedBuilder) GetLastCompletedBuild() (*BuildResult, error) {
	data, err := f.readObject("latest-build.txt")
	if err != nil {
		return nil, err
	}
	glog.V(8).Infof("Got data: %s", string(data))

	fields, err := parseDelimited(data)
	if err != nil {
		return nil, fmt.Errorf("error parsing latest-build file for %s: %v", f.name, err)
	}

	buildID := fields["BUILD_NUMBER"]
	//gitCommit := fields["GIT_COMMIT"]

	if buildID == "" {
		return nil, nil
	}

	return f.readBuildResult(buildID)
}

// Parses a text file comprising lines of `key=value`, putting the result into a map.
// Invalid lines are ignored.  For duplicated values only the last value will be returned.
func parseDelimited(data []byte) (map[string]string, error) {
	fields := make(map[string]string)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 {
			glog.Warningf("Ignoring line that did not match expected format: %q", line)
			continue
		}

		fields[tokens[0]] = tokens[1]
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return fields, nil
}
