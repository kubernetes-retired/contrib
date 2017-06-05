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

package pkg

type Package string

const NullPackage Package = ""

type Path []Package

func (p Path) Last() Package {
	return p[len(p)-1]
}

func (p Path) Pop() Path {
	return p[:len(p)-1]
}

type present struct{}

type Set map[Package]present

func NewSet(pkgs ...Package) Set {
	set := make(Set, len(pkgs))
	for _, pkg := range pkgs {
		set[pkg] = present{}
	}
	return set
}

func (ps Set) Insert(pkg Package) {
	ps[pkg] = present{}
}

func (ps Set) Delete(pkg Package) {
	delete(ps, pkg)
}

func (ps Set) Has(pkg Package) bool {
	_, found := ps[pkg]
	return found
}
