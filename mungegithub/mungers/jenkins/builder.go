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

// BuildResult is the result of an individual build (typically a test run)
type BuildResult struct {
	Success bool
	BuildID string
}

// IsStable is really is success, but maybe there is a way to make it look
// at multiple runs...
func (r BuildResult) IsStable() bool {
	return r.Success
}

// Builder represents something that runs builds and reports results
// For example, a direct Jenkins server, or a storage bucket containing build
// results from a federated builder.
type Builder interface {
	GetLastCompletedBuild() (*BuildResult, error)
}
