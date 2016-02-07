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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"golang.org/x/net/context"

	"github.com/golang/glog"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

// BuilderConfig contains the configuration settings for reading build results from a Builder
type BuilderConfig struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Gating bool   `json:"gating"`
}

// FederatedBuilder is how we talk to the repository of build results
type FederatedBuilder struct {
	bucketName string
	basePath   string

	name   string
	config *BuilderConfig

	ctx    context.Context
	client *storage.Client
	bucket *storage.BucketHandle
}

var _ Builder = &FederatedBuilder{}

// NewFederatedBuilder is a constructor for FederatedBuilder objects
func NewFederatedBuilder(config *BuilderConfig) (*FederatedBuilder, error) {
	path := config.Path
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
		name:       config.Name,
		config:     config,
	}
	return f, nil
}

// Close releases internal state
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

// buildResultFile is the format of finished.json, as uploaded by the builders.
type buildResultFile struct {
	Result    string `json:"result"`
	Timestamp uint64 `json:"timestamp"`
}

// Relies on a build result file in `<buildID>/finished.json`.
// The build result should be JSON file in the format defined by buildResultFile.
func (f *FederatedBuilder) readBuildResult(buildID string) (*BuildResult, error) {
	data, err := f.readObject(buildID + "/finished.json")
	if err != nil {
		return nil, err
	}

	humanName := f.name + ":" + buildID

	glog.V(8).Infof("Got data: %s", string(data))

	parsed := &buildResultFile{}
	err = json.Unmarshal(data, parsed)
	if err != nil {
		return nil, fmt.Errorf("error parsing build result for %s: %v", humanName, err)
	}

	success := false
	if parsed.Result == "SUCCESS" {
		success = true
	}

	br := &BuildResult{
		Success: success,
		BuildID: buildID,
	}

	return br, nil
}

// GetLastCompletedBuild does just that
//
// Relies on a text file `latest-build.txt`, which should be a plain text file
// containing the ID of the latest build.
func (f *FederatedBuilder) GetLastCompletedBuild() (*BuildResult, error) {
	data, err := f.readObject("latest-build.txt")
	if err != nil {
		return nil, err
	}
	glog.V(8).Infof("Got data: %s", string(data))

	buildID := string(data)

	buildID = strings.TrimSpace(buildID)

	if buildID == "" {
		return nil, nil
	}

	return f.readBuildResult(buildID)
}
