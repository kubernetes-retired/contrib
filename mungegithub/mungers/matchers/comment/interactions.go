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
	"regexp"
	"strings"

	"github.com/google/go-github/github"
)

// Notification identifies comments with the following format:
// [NEEDS-REBASE] Optional arguments
type NotificationName string

// Match returns true if the comment is a notification
func (b NotificationName) Match(comment *github.IssueComment) bool {
	notif := ParseNotification(comment)
	if notif == nil {
		return false
	}

	return strings.ToUpper(notif.Name) == strings.ToUpper(string(b))
}

// Command identifies messages sent to the bot, with the following format:
// /COMMAND Optional arguments
type CommandName string

// Match will return true if the comment is indeed a command
func (c CommandName) Match(comment *github.IssueComment) bool {
	command := ParseCommand(comment)
	if command == nil {
		return false
	}
	return strings.ToUpper(command.Name) == strings.ToUpper(string(c))
}

type CommandArguments regexp.Regexp

func (c CommandArguments) Match(comment *github.IssueComment) bool {
	command := ParseCommand(comment)
	if command == nil {
		return false
	}
	return (*regexp.Regexp)(&c).MatchString(command.Arguments)
}

func MungeBotAuthor() Matcher {
	return AuthorLogin("k8s-merge-robot")
}

func JenkinsBotAuthor() Matcher {
	return AuthorLogin("k8s-bot")
}

func BotAuthor() Matcher {
	return Or([]Matcher{
		MungeBotAuthor(),
		JenkinsBotAuthor(),
	})
}

func HumanActor() Matcher {
	return And([]Matcher{
		ValidAuthor{},
		Not{BotAuthor()},
	})
}

func MungerNotificationName(notif string) Matcher {
	return And([]Matcher{
		MungeBotAuthor(),
		NotificationName(notif),
	})
}
