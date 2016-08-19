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

package authreq

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/kubernetes/pkg/apis/extensions"
)

const (
	// external URL that provides the authentication
	authURL = "ingress.kubernetes.io/auth-url"
)

var (
	// ErrMissingAnnotations is returned when the ingress rule
	// does not contains annotations related with authentication
	ErrMissingAnnotations = errors.New("missing authentication annotations")
)

type ingAnnotations map[string]string

func (a ingAnnotations) url() (string, error) {
	val, ok := a[authURL]
	if !ok {
		return "", ErrMissingAnnotations
	}

	return val, nil
}

// ParseAnnotations parses the annotations contained in the ingress
// rule used to use an external URL as source for authentication
func ParseAnnotations(ing *extensions.Ingress) (string, error) {
	if ing.GetAnnotations() == nil {
		return "", ErrMissingAnnotations
	}

	str, err := ingAnnotations(ing.GetAnnotations()).url()
	if err != nil {
		return "", err
	}

	if str == "" {
		return "", fmt.Errorf("an empty string is not a valid URL")
	}

	ur, err := url.Parse(str)
	if err != nil {
		return "", err
	}
	if ur.Scheme == "" {
		return "", fmt.Errorf("url scheme is empty")
	}
	if ur.Host == "" {
		return "", fmt.Errorf("url host is empty")
	}

	if strings.Index(ur.Host, "..") != -1 {
		return "", fmt.Errorf("invalid url host")
	}

	return str, nil
}
