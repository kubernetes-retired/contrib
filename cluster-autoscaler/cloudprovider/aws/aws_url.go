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

package aws

import (
	"fmt"
	"strings"
)

const (
	awsUrlSchema        = "https"
	awsDomainSufix      = "googleapis.com/compute/v1/projects/"
	awsPrefix           = awsUrlSchema + "://content." + awsDomainSufix
	instanceUrlTemplate = awsPrefix + "%s/zones/%s/instances/%s"
	asgUrlTemplate      = awsPrefix + "%s/zones/%s/instanceGroups/%s"
)

// ParseAsgUrl expects url in format:
// https://content.googleapis.com/compute/v1/projects/<project-id>/zones/<zone>/instanceGroups/<name>
func ParseAsgUrl(url string) (project string, zone string, name string, err error) {
	return parseAwsUrl(url, "instanceGroups")
}

// ParseInstanceUrl expects url in format:
// https://content.googleapis.com/compute/v1/projects/<project-id>/zones/<zone>/instances/<name>
func ParseInstanceUrl(url string) (project string, zone string, name string, err error) {
	return parseAwsUrl(url, "instances")
}

// GenerateInstanceUrl generates url for instance.
func GenerateInstanceUrl(project, zone, name string) string {
	return fmt.Sprintf(instanceUrlTemplate, project, zone, name)
}

// GenerateAsgUrl generates url for instance.
func GenerateAsgUrl(project, zone, name string) string {
	return fmt.Sprintf(asgUrlTemplate, project, zone, name)
}

func parseAwsUrl(url, expectedResource string) (project string, zone string, name string, err error) {
	errMsg := fmt.Errorf("Wrong url: expected format https://content.googleapis.com/compute/v1/projects/<project-id>/zones/<zone>/%s/<name>, got %s", expectedResource, url)
	if !strings.Contains(url, awsDomainSufix) {
		return "", "", "", errMsg
	}
	if !strings.HasPrefix(url, awsUrlSchema) {
		return "", "", "", errMsg
	}
	splitted := strings.Split(strings.Split(url, awsDomainSufix)[1], "/")
	if len(splitted) != 5 || splitted[1] != "zones" {
		return "", "", "", errMsg
	}
	if splitted[3] != expectedResource {
		return "", "", "", fmt.Errorf("Wrong resource in url: expected %s, got %s", expectedResource, splitted[3])
	}
	project = splitted[0]
	zone = splitted[2]
	name = splitted[4]
	return project, zone, name, nil
}
