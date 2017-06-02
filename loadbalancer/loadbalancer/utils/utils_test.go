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

package utils

import (
	"sort"
	"testing"
)

var testMap1 = map[string]string{
	"group0.key1": "value01",
	"group1.key1": "value11",
	"group1.key2": "value12",
	"group2.key1": "value21",
}

var testMap2 = map[string]string{
	"group0.key1": "value01",
	"group1.key1": "value11",
	"group1.key2": "value120",
	"group3.key1": "value31",
}

func TestGetGroupName(t *testing.T) {
	group := getGroupName("group1.key1")
	if group != "group1" {
		t.Errorf("In getGroupName(), expected group1. Got %s.", group)
	}
}

func TestGetConfigMapGroups(t *testing.T) {
	groups := GetConfigMapGroups(testMap1)
	if groups.Len() != 3 {
		t.Errorf("In getConfigMapGroups(), expected length %v. Got %v.", 2, groups.Len())
	}
	if !groups.HasAll("group0", "group1", "group2") {
		t.Errorf("In getGroupName(), expected group0, group1 and group2. Got %v.", groups.List)
	}
}

func TestMapKeys(t *testing.T) {
	expected := []string{"group0.key1", "group1.key1", "group1.key2", "group2.key1"}
	keys := MapKeys(testMap1)
	sort.Strings(keys)
	for i := 0; i < len(expected); i++ {
		if keys[i] != expected[i] {
			t.Fatalf("Keys dont match. Expected %v. Got %v.", expected, keys)
		}
	}
}

func TestGetConfigMapDiff(t *testing.T) {
	diff := getConfigMapDiff(testMap1, testMap2)

	for _, d := range diff {
		switch d.key {
		case "group1.key2":
			if d.a != "value12" {
				t.Errorf("Expected value12 for key %s. Got %s.", d.key, d.a)
			}
			if d.b != "value120" {
				t.Errorf("Expected value120 for key %s. Got %s.", d.key, d.b)
			}
		case "group2.key1":
			if d.a != "value21" {
				t.Errorf("Expected value21 for key %s. Got %s.", d.key, d.a)
			}
			if d.b != "" {
				t.Errorf("Expected empty string for key %s. Got %s.", d.key, d.b)
			}
		case "group3.key1":
			if d.a != "" {
				t.Errorf("Expected empty string for key %s. Got %s.", d.key, d.a)
			}
			if d.b != "value31" {
				t.Errorf("Expected value31 for key %s. Got %s.", d.key, d.b)
			}
		default:
			panic("unrecognized escape character")
		}
	}
}

func TestGetUpdatedConfigMapGroups(t *testing.T) {
	updatedGroups := GetUpdatedConfigMapGroups(testMap1, testMap2)
	if !updatedGroups.HasAll("group1", "group2", "group3") {
		t.Errorf("In getGroupName(), expected group1, group2 and group3. Got %v.", updatedGroups.List)
	}
}
