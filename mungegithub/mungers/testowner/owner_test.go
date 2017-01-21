/*
Copyright 2016 The Kubernetes Authors.

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

package testowner

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"A":                                    "a",
		"Perf [Performance]":                   "perf",
		"[k8s.io] test [performance] stuff":    "test stuff",
		"[k8s.io] blah {Kubernetes e2e suite}": "blah",
	}
	for input, output := range tests {
		result := normalize(input)
		if result != output {
			t.Errorf("normalize(%s) != %s (got %s)", input, output, result)
		}
	}
}

func TestOwnerList(t *testing.T) {
	list := NewOwnerList(map[string]*Owner{"Perf [performance]": {
		User: "me",
		SIG:  "group",
	}})
	owner := list.TestOwner("perf [flaky]")
	if owner.User != "me" {
		t.Error("Unexpected return value ", owner)
	}
	owner = list.TestOwner("Unknown test")
	if owner != nil {
		t.Errorf("Unexpected return value ", owner)
	}
}

func TestOwnerGlob(t *testing.T) {
	list := NewOwnerList(map[string]*Owner{"blah * [performance] test *": {
		User: "me",
		SIG:  "group",
	}})
	owner := list.TestOwner("blah 200 test foo")
	if owner.User != "me" {
		t.Error("Unexpected return value ", owner)
	}
	if owner.SIG != "group" {
		t.Error("Unexpected return value ", owner)
	}
	owner = list.TestOwner("Unknown test")
	if owner != nil {
		t.Errorf("Unexpected return value ", owner)
	}
}

func TestOwnerListDefault(t *testing.T) {
	list := NewOwnerList(map[string]*Owner{"DEFAULT": {
		User: "elves",
		SIG:  "group",
	}})
	owner := list.TestOwner("some random new test")
	if owner.User != "elves" {
		t.Error("Unexpected return value ", owner)
	}
	if owner.SIG != "group" {
		t.Error("Unexpected return value ", owner)
	}
}

func TestOwnerListRandom(t *testing.T) {
	list := NewOwnerList(map[string]*Owner{"testname": {
		User: "a/b/c/d",
	}})
	counts := map[string]int{"a": 0, "b": 0, "c": 0, "d": 0}
	for i := 0; i < 1000; i++ {
		counts[list.TestOwner("testname").User]++
	}
	for name, count := range counts {
		if count <= 200 {
			t.Errorf("Too few assigments to %s: only %d, expected > 200", name, count)
		}
	}
}

func TestOwnerListFromCsv(t *testing.T) {
	r := bytes.NewReader([]byte(",,,header nonsense,\n" +
		",owner,suggested owner,name,sig\n" +
		",foo,other,Test name,Node\n" +
		",bar,foo,other test,Windows\n"))
	list, err := NewOwnerListFromCsv(r)
	if err != nil {
		t.Error(err)
	}
	if owner := list.TestOwner("test name"); owner.User != "foo" {
		t.Error("unexpected return value ", owner)
	} else if owner.SIG != "Node" {
		t.Error("unexpected return value ", owner)
	}
	if owner := list.TestOwner("other test"); owner.User != "bar" {
		t.Error("unexpected return value ", owner)
	} else if owner.SIG != "Windows" {
		t.Error("unexpected return value ", owner)
	}
}

func TestReloadingOwnerList(t *testing.T) {
	tempfile, err := ioutil.TempFile(os.TempDir(), "ownertest")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(tempfile.Name())
	defer tempfile.Close()
	writer := bufio.NewWriter(tempfile)
	_, err = writer.WriteString("owner,name,sig\nfoo,flake,Scheduling\n")
	if err != nil {
		t.Error(err)
	}
	err = writer.Flush()
	if err != nil {
		t.Error(err)
	}
	list, err := NewReloadingOwnerList(tempfile.Name())
	if err != nil {
		t.Error(err)
	}
	if owner := list.TestOwner("flake"); owner.User != "foo" {
		t.Error("unexpected owner for 'flake': ", owner)
	} else if owner.SIG != "Scheduling" {
		t.Error("unexpected owner for 'flake': ", owner)
	}

	// Assuming millisecond resolution on our FS, this sleep
	// ensures the mtime will change with the next write.
	time.Sleep(5 * time.Millisecond)

	// Clear file and reset writing offset
	tempfile.Truncate(0)
	tempfile.Seek(0, os.SEEK_SET)
	writer.Reset(tempfile)
	_, err = writer.WriteString("owner,name,sig\nbar,flake,AWS\n")
	if err != nil {
		t.Error(err)
	}
	err = writer.Flush()
	if err != nil {
		t.Error(err)
	}

	if owner := list.TestOwner("flake"); owner.User != "bar" {
		t.Error("unexpected owner for 'flake': ", owner)
	} else if owner.SIG != "AWS" {
		t.Error("unexpected owner for 'flake': ", owner)
	}
}
