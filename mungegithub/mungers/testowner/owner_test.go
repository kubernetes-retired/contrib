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

package testowner

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"testing"
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
	list := NewOwnerList(map[string]string{"Perf [performane]": "me"})
	owner := list.TestOwner("perf [flaky]")
	if owner != "me" {
		t.Error("Unexpected return value ", owner)
	}
	owner = list.TestOwner("Unknown test")
	if owner != "" {
		t.Errorf("Unexpected return value ", owner)
	}
}

func TestOwnerListFromCsv(t *testing.T) {
	r := bytes.NewReader([]byte(",,header nonsense,\n" +
		",owner,suggested owner,test name\n" +
		",foo,other,Test name\n" +
		",bar,foo,other test\n"))
	list, err := NewOwnerListFromCsv(r)
	if err != nil {
		t.Error(err)
	}
	if owner := list.TestOwner("test name"); owner != "foo" {
		t.Error("unexpected return value ", owner)
	}
	if owner := list.TestOwner("other test"); owner != "bar" {
		t.Error("unexpected return value ", owner)
	}
}

type DataServer struct {
	data  string
	calls int
}

func (t *DataServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dataHash := fmt.Sprintf("%d", crc32.ChecksumIEEE([]byte(t.data)))
	if r.Header.Get("If-None-Match") == dataHash {
		w.WriteHeader(304)
		return
	}
	t.calls++
	w.Header().Add("ETag", dataHash)
	w.Write([]byte(t.data))
}

func TestHTTPOwnerList(t *testing.T) {
	handler := DataServer{data: "owner,test name\nfoo,flake\n"}

	server := httptest.NewServer(&handler)
	defer server.Close()

	list, err := NewHTTPOwnerList(server.URL)

	if err != nil {
		t.Error(err)
	}

	if owner := list.TestOwner("flake"); owner != "foo" {
		t.Error("unexpected owner for 'flake': ", owner)
	}

	handler.data = "owner,test name\nbar,flake\n"

	if owner := list.TestOwner("flake"); owner != "bar" {
		t.Error("unexpected owner for 'flake': ", owner)
	}

	// Verify that the httpcache is preventing excessive modifications.
	for i := 0; i < 10; i++ {
		list.TestOwner("flake")
	}

	if handler.calls != 2 {
		t.Error("expected only 2 http requests, got ", handler.calls)
	}
}
