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

package mungers

import (
	"reflect"
	"runtime"
	"testing"

	"k8s.io/kubernetes/pkg/util/sets"
)

var (
	aliasYaml = `
aliases:
  team/t1:
    - u1
    - u2`

	aliasRecursiveYaml = `
aliases:
  team/t1:
    - u1
    - u2
  team/t2:
    - team/t1
    - u3`

	aliasRecursiveLoopYaml = `
aliases:
  team/t1:
    - team/t2
    - u2
  team/t2:
    - team/t3
    - u3
  team/t3:
    - u4`
)

func TestExpandAliases(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	tests := []struct {
		name      string
		aliasFile string
		owners    sets.String
		expected  sets.String
	}{
		{
			name:      "No expansions.",
			aliasFile: aliasYaml,
			owners:    sets.NewString("abc", "def"),
			expected:  sets.NewString("abc", "def"),
		},
		{
			name:      "No aliases to be expanded",
			aliasFile: aliasYaml,
			owners:    sets.NewString("abc", "team/t1"),
			expected:  sets.NewString("abc", "u1", "u2"),
		},
		{
			name:      "Duplicates inside and outside alias.",
			aliasFile: aliasYaml,
			owners:    sets.NewString("u1", "team/t1"),
			expected:  sets.NewString("u1", "u2"),
		},
		{
			name:      "Recursive aliases",
			aliasFile: aliasRecursiveYaml,
			owners:    sets.NewString("team/t2"),
			expected:  sets.NewString("u1", "u2", "u3"),
		},
		{
			name:      "Recursive many levels",
			aliasFile: aliasRecursiveLoopYaml,
			owners:    sets.NewString("team/t1"),
			expected:  sets.NewString("u2", "u3", "u4"),
		},
	}

	for _, test := range tests {
		b := BlunderbussMunger{}
		b.AliasFile = "file" // because we expect a file to be provided.
		b.aliasReadFunc = func() ([]byte, error) {
			return []byte(test.aliasFile), nil
		}
		if err := b.EachLoop(); err != nil {
			t.Fatalf("%v", err)
		}

		expanded := b.expandAliases(test.owners)
		if !reflect.DeepEqual(expanded, test.expected) {
			t.Errorf("%s: expected: %#v, got: %#v", test.name, test.expected, expanded)
		}
	}
}
