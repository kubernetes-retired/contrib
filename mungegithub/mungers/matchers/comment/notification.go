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

package comment

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/github"
	mgh "k8s.io/contrib/mungegithub/github"
)

type Notification struct {
	Name      string
	Arguments string
	Context   string
}

var (
	notificationRegex = regexp.MustCompile(`^\[([^\]\s]+)\]\s?(.*)$`)
)

// ParseNotification attempts to read a notification from a comment
// Returns nil if the comment doesn't contain a notification
// Also note that Context is not parsed from the notification
func ParseNotification(comment *github.IssueComment) *Notification {
	if comment == nil || comment.Body == nil {
		return nil
	}

	match := notificationRegex.FindStringSubmatch(*comment.Body)
	if match == nil {
		return nil
	}

	return &Notification{
		Name:      strings.ToUpper(match[1]),
		Arguments: strings.TrimSpace(match[2]),
	}
}

// String converts the notification
func (n *Notification) String() string {
	str := fmt.Sprintf("[%s] %s", strings.ToUpper(n.Name), strings.TrimSpace(n.Arguments))

	if n.Context != "" {
		str += "\n\n" + n.Context
	}

	return str
}

// Post a new notification on Github
func (n Notification) Post(obj *mgh.MungeObject) error {
	return obj.WriteComment(n.String())
}
