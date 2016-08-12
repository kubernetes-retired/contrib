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
	"testing"

	"github.com/google/go-github/github"
)

func makeCommentWithBody(body string) *github.IssueComment {
	return &github.IssueComment{
		Body: &body,
	}
}

func TestBotMessage(t *testing.T) {
	if BotMessage("MESSAGE").Match(&github.IssueComment{}) {
		t.Error("Shouldn't matches nil body")
	}
	if BotMessage("MESSAGE").Match(makeCommentWithBody("MESSAGE WRONG FORMAT")) {
		t.Error("Invalid match")
	}
	if !BotMessage("MESSAGE").Match(makeCommentWithBody("[MESSAGE] Valid format")) {
		t.Error("Should have matched")
	}
	if !BotMessage("MESSAGE").Match(makeCommentWithBody("[MESSAGE]")) {
		t.Error("Doesn't match with no arguments")
	}
}

func TestCommand(t *testing.T) {
	if Command("COMMAND").Match(&github.IssueComment{}) {
		t.Error("Shouldn't matches nil body")
	}
	if Command("COMMAND").Match(makeCommentWithBody("COMMAND WRONG FORMAT")) {
		t.Error("Invalid match")
	}
	if !Command("COMMAND").Match(makeCommentWithBody("/COMMAND Valid format")) {
		t.Error("Should have matched")
	}
	if !Command("COMMAND").Match(makeCommentWithBody("/COMMAND")) {
		t.Error("Doesn't match with no arguments")
	}
}
