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

package nanny

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Scaler determines the number of replicas to run
type Scaler struct {
	ConfigFile string
	Verbose    bool
}

const paramsSeparator = "nodes.gte."

func (s Scaler) scaleWithNodes(numCurrentNodes uint64) uint64 {
	newMap, err := ParseScalerParamsFile(s.ConfigFile)
	if err != nil {
		log.Fatalf("Parse failure: The configmap volume file is malformed (%s)\n", err)
	}

	// construct a search ladder from the map
	keys := make([]int, 0, len(newMap))
	for k := range newMap {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	var neededReplicas uint64 = 1

	// Walk the search ladder to get the correct number of replicas
	// for the current number of nodes
	for _, nodeCount := range keys {
		replicas := newMap[uint64(nodeCount)]
		if uint64(nodeCount) > numCurrentNodes {
			break
		}
		neededReplicas = replicas
	}
	return neededReplicas
}

// ParseScalerParamsFile Parse the scaler params file every time around, since it is a ConfigMap volume.
func ParseScalerParamsFile(filename string) (map[uint64]uint64, error) {
	NodeToReplicasMap := make(map[uint64]uint64)
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Params file %s does not exist", filename)
		}
		return nil, fmt.Errorf("Params file %s is not readable", filename)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var n = 1
	for scanner.Scan() {
		l := scanner.Text()
		// Each line should be of the form nodes.gte.<N>=<M>
		parts := strings.Split(l, "=")
		if len(parts) < 2 {
			return nil, fmt.Errorf("Error parsing line %d in params file %s", n, filename)
		}
		tokens := strings.Split(parts[0], paramsSeparator)
		if len(tokens) < 2 {
			return nil, fmt.Errorf("Error parsing line %d in params file %s", n, filename)
		}
		node, err := strconv.ParseUint(tokens[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error parsing line %d in params file %s", n, filename)
		}
		desiredReplicaCount, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error parsing line %d in params file %s", n, filename)
		}
		NodeToReplicasMap[node] = desiredReplicaCount
		n++
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error %s reading params file", err)
	}
	return NodeToReplicasMap, nil
}
