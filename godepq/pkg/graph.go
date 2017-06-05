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

type Graph map[Package]Set

func NewGraph() Graph {
	return make(Graph)
}

func (pg Graph) Pkg(pkg Package) Set {
	if set, ok := pg[pkg]; ok {
		return set
	}
	set := NewSet()
	pg[pkg] = set
	return set
}

func (pg Graph) Has(pkg Package) bool {
	_, found := pg[pkg]
	return found
}

// AddPath inserts the path into the graph.
func (pg Graph) AddPath(path Path) {
	var last Set
	for _, pkg := range path {
		if last != nil {
			last.Insert(pkg)
		}
		last = pg.Pkg(pkg)
	}
}

func (pg Graph) SomePath(start, end Package) Path {
	if _, ok := pg[start]; !ok {
		return nil
	} else if _, ok := pg[end]; !ok {
		return nil
	}

	var fullPath Path
	walkFn := func(pkg Package, _ Set, path Path) bool {
		if pkg == end {
			fullPath = path
			return false
		}
		return true
	}
	pg.DepthFirst(start, walkFn)
	return fullPath
}

func (pg Graph) AllPaths(start, end Package) Graph {
	if _, ok := pg[start]; !ok {
		return nil
	} else if _, ok := pg[end]; !ok {
		return nil
	}

	paths := NewGraph()
	walkFn := func(pkg Package, edges Set, path Path) bool {
		if pkg == end {
			paths.AddPath(path)
			return true
		}
		for edge := range edges {
			if paths.Has(edge) {
				paths.AddPath(append(path, edge))
			}
		}
		return true
	}
	pg.DepthFirst(start, walkFn)
	return paths
}

type WalkFn func(pkg Package, edges Set, path Path) (keepGoing bool)

// Walk the graph depth first, starting at start and calling walkFn on each node visited.
// Each node will be visited at most once.
func (pg Graph) DepthFirst(start Package, walkFn WalkFn) {
	if _, ok := pg[start]; !ok {
		return
	}

	path := Path{start}
	if !walkFn(start, pg[start], path) {
		return
	}

	visited := NewSet(start)
walk:
	for len(path) > 0 {
		for pkg := range pg[path.Last()] {
			if !visited.Has(pkg) {
				path = append(path, pkg)
				if !walkFn(pkg, pg[pkg], path) {
					return
				}
				visited.Insert(pkg)
				continue walk
			}
		}
		path = path.Pop() // Backtrack.
	}
}

// Walk the graph "depth last", starting at start and calling walkFn on each node visited.  Each
// node will be visited at most once. Nodes will be visited "depth last", where depth is defined as
// the maximum distance from the start.
// TODO: (if needed) add path to WalkFn
func (pg Graph) DepthLast(start Package, walkFn WalkFn) {
	if _, ok := pg[start]; !ok {
		return
	}

	// First, build the depth map.
	// TODO: there's probably a more efficient algorithm than this
	depths := map[Package]int{
		start: 0,
	}
	visited := NewSet()
	type depthPair struct {
		p Package
		d int
	}
	maxDepth := 0
	queue := []depthPair{{start, 0}}
	for len(queue) > 0 {
		dp := queue[0]
		queue = queue[1:]

		if dp.d < depths[dp.p] || (visited.Has(dp.p) && dp.d == depths[dp.p]) {
			continue
		}
		visited.Insert(dp.p)
		depths[dp.p] = dp.d
		for pkg := range pg[dp.p] {
			queue = append(queue, depthPair{pkg, dp.d + 1})
		}
		if maxDepth < dp.d {
			maxDepth = dp.d
		}
	}

	for i := 0; i <= maxDepth; i++ {
		for pkg, depth := range depths {
			if depth == i {
				if !walkFn(pkg, pg[pkg], nil) {
					return
				}
			}
		}
	}
}
