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

import (
	"errors"
	"go/build"
	"log"
	"regexp"
)

type Dependencies struct {
	// Map of package -> dependencies.
	Forward Graph
	// Packages which were ignored.
	Ignored Set
}

type Condition func(Dependencies) bool

type Builder struct {
	// The base directory for relative imports.
	BaseDir string
	// The roots of the dependency graph (source packages).
	Roots []Package
	// Stop building the graph if ANY conditions are met.
	TerminationConditions []Condition
	// Ignore any packages that match any of these patterns.
	Ignored []*regexp.Regexp
	// Whether tests should be included in the dependencies.
	IncludeTests bool
	// Whether to include standard library packages
	IncludeStdlib bool
	// The build context for processing imports.
	BuildContext build.Context

	// Internal
	deps Dependencies
}

// Packages which should always be ignored.
var pkgBlacklist = NewSet(
	"C", // c imports, causes problems
)

func (b *Builder) Build() (Dependencies, error) {
	b.deps = Dependencies{
		Forward: NewGraph(),
		Ignored: NewSet(),
	}

	err := b.addAllPackages(b.Roots)
	if err == termination {
		err = nil // Ignore termination condition.
	}
	return b.deps, err
}

func (b *Builder) addAllPackages(pkgs []Package) error {
	for _, pkg := range pkgs {
		if b.isIgnored(pkg) {
			log.Printf("Warning: ignoring root package %q", pkg)
			b.deps.Ignored.Insert(pkg)
			continue
		}

		// TODO: add support for recursive sub-packages.
		if err := b.addPackage(pkg); err != nil {
			return err
		}
	}
	return nil
}

var termination = errors.New("termination condition met")

// Recursively adds a package to the accumulated dependency graph.
func (b *Builder) addPackage(pkgName Package) error {
	pkg, err := b.BuildContext.Import(string(pkgName), b.BaseDir, 0)
	if err != nil {
		return err
	}

	pkgFullName := Package(pkg.ImportPath)
	if b.isIgnored(pkgFullName) {
		b.deps.Ignored.Insert(pkgFullName)
		return nil
	}

	// Insert the package.
	b.deps.Forward.Pkg(pkgFullName)

	for _, condition := range b.TerminationConditions {
		if condition(b.deps) {
			return termination
		}
	}

	isStdlib := pkg.Goroot
	if isStdlib && !b.IncludeStdlib {
		return nil // TODO - do we need to do anything else here?
	}

	for _, imp := range b.getImports(pkg) {
		if b.isIgnored(imp) {
			b.deps.Ignored.Insert(pkgFullName)
			continue
		}

		b.deps.Forward.Pkg(pkgFullName).Insert(imp)

		// If imp has not been added, add it now.
		if !b.deps.Forward.Has(imp) {
			if err := b.addPackage(imp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *Builder) getImports(pkg *build.Package) []Package {
	allImports := pkg.Imports
	if b.IncludeTests {
		allImports = append(allImports, pkg.TestImports...)
		allImports = append(allImports, pkg.XTestImports...)
	}
	var imports []Package
	found := make(map[string]struct{})
	for _, imp := range allImports {
		if imp == pkg.ImportPath {
			// Don't draw a self-reference when foo_test depends on foo.
			continue
		}
		if _, ok := found[imp]; ok {
			continue
		}
		found[imp] = struct{}{}
		imports = append(imports, Package(imp))
	}
	return imports
}

func (b *Builder) isIgnored(pkg Package) bool {
	if pkgBlacklist.Has(pkg) {
		return true
	}
	for _, r := range b.Ignored {
		if r.MatchString(string(pkg)) {
			return true
		}
	}
	return false
}
