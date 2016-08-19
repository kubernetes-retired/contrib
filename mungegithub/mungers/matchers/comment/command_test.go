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
	"reflect"
	"testing"

	"github.com/google/go-github/github"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		command *Command
		comment string
	}{
		{
			command: nil,
			comment: "I have nothing to do with a command",
		},
		{
			command: nil,
			comment: " /COMMAND Line can't start with spaces",
		},
		{
			command: &Command{Name: "COMMAND", Arguments: "Valid command"},
			comment: "/COMMAND Valid command",
		},
		{
			command: &Command{Name: "COMMAND", Arguments: "Command name is upper-cased"},
			comment: "/command Command name is upper-cased",
		},
		{
			command: &Command{Name: "COMMAND", Arguments: "Arguments is trimmed"},
			comment: "/COMMAND     Arguments is trimmed   ",
		},
	}

	for _, test := range tests {
		actualCommand := ParseCommand(&github.IssueComment{Body: &test.comment})
		if !reflect.DeepEqual(actualCommand, test.command) {
			t.Error(actualCommand, "doesn't match expected command:", test.command)
		}
	}
}

func TestStringCommand(t *testing.T) {
	tests := []struct {
		command *Command
		str     string
	}{
		{
			command: &Command{Name: "COMMAND", Arguments: "Argument"},
			str:     "/COMMAND Argument",
		},
		{
			command: &Command{Name: "command", Arguments: "  Argument  "},
			str:     "/COMMAND Argument",
		},
	}

	for _, test := range tests {
		actualString := test.command.String()
		if actualString != test.str {
			t.Error(actualString, "doesn't match expected string:", test.str)
		}
	}
}
