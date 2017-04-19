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

package garbagecollector

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	_ "k8s.io/kubernetes/pkg/api/install"

	"github.com/stretchr/testify/assert"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta/metatypes"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/dynamic"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/json"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/workqueue"
)

func TestNewGarbageCollector(t *testing.T) {
	clientPool := dynamic.NewClientPool(&restclient.Config{}, dynamic.LegacyAPIPathResolverFunc)
	podResource := []unversioned.GroupVersionResource{{Version: "v1", Resource: "pods"}}
	gc, err := NewGarbageCollector(clientPool, podResource)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(gc.monitors))
}

// fakeAction records information about requests to aid in testing.
type fakeAction struct {
	method string
	path   string
	query  string
}

// String returns method=path to aid in testing
func (f *fakeAction) String() string {
	return strings.Join([]string{f.method, f.path}, "=")
}

type FakeResponse struct {
	statusCode int
	content    []byte
}

// fakeActionHandler holds a list of fakeActions received
type fakeActionHandler struct {
	// statusCode and content returned by this handler for different method + path.
	response map[string]FakeResponse

	lock    sync.Mutex
	actions []fakeAction
}

// ServeHTTP logs the action that occurred and always returns the associated status code
func (f *fakeActionHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.actions = append(f.actions, fakeAction{method: request.Method, path: request.URL.Path, query: request.URL.RawQuery})
	fakeResponse, ok := f.response[request.Method+request.URL.Path]
	if !ok {
		fakeResponse.statusCode = 200
		fakeResponse.content = []byte("{\"kind\": \"List\"}")
	}
	response.WriteHeader(fakeResponse.statusCode)
	response.Write(fakeResponse.content)
}

// testServerAndClientConfig returns a server that listens and a config that can reference it
func testServerAndClientConfig(handler func(http.ResponseWriter, *http.Request)) (*httptest.Server, *restclient.Config) {
	srv := httptest.NewServer(http.HandlerFunc(handler))
	config := &restclient.Config{
		Host: srv.URL,
	}
	return srv, config
}

func newDanglingPod() *v1.Pod {
	return &v1.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ToBeDeletedPod",
			Namespace: "ns1",
			OwnerReferences: []v1.OwnerReference{
				{
					Kind:       "ReplicationController",
					Name:       "owner1",
					UID:        "123",
					APIVersion: "v1",
				},
			},
		},
	}
}

// test the processItem function making the expected actions.
func TestProcessItem(t *testing.T) {
	pod := newDanglingPod()
	podBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	testHandler := &fakeActionHandler{
		response: map[string]FakeResponse{
			"GET" + "/api/v1/namespaces/ns1/replicationcontrollers/owner1": {
				404,
				[]byte{},
			},
			"GET" + "/api/v1/namespaces/ns1/pods/ToBeDeletedPod": {
				200,
				podBytes,
			},
		},
	}
	podResource := []unversioned.GroupVersionResource{{Version: "v1", Resource: "pods"}}
	srv, clientConfig := testServerAndClientConfig(testHandler.ServeHTTP)
	defer srv.Close()
	clientPool := dynamic.NewClientPool(clientConfig, dynamic.LegacyAPIPathResolverFunc)
	gc, err := NewGarbageCollector(clientPool, podResource)
	if err != nil {
		t.Fatal(err)
	}
	item := &node{
		identity: objectReference{
			OwnerReference: metatypes.OwnerReference{
				Kind:       pod.Kind,
				APIVersion: pod.APIVersion,
				Name:       pod.Name,
				UID:        pod.UID,
			},
			Namespace: pod.Namespace,
		},
		// owners are intentionally left empty. The processItem routine should get the latest item from the server.
		owners: nil,
	}
	err = gc.processItem(item)
	if err != nil {
		t.Errorf("Unexpected Error: %v", err)
	}
	expectedActionSet := sets.NewString()
	expectedActionSet.Insert("GET=/api/v1/namespaces/ns1/replicationcontrollers/owner1")
	expectedActionSet.Insert("DELETE=/api/v1/namespaces/ns1/pods/ToBeDeletedPod")
	expectedActionSet.Insert("GET=/api/v1/namespaces/ns1/pods/ToBeDeletedPod")

	actualActionSet := sets.NewString()
	for _, action := range testHandler.actions {
		actualActionSet.Insert(action.String())
	}
	if !expectedActionSet.Equal(actualActionSet) {
		t.Errorf("expected actions:\n%v\n but got:\n%v\nDifference:\n%v", expectedActionSet,
			actualActionSet, expectedActionSet.Difference(actualActionSet))
	}
}

// verifyGraphInvariants verifies that all of a node's owners list the node as a
// dependent and vice versa. uidToNode has all the nodes in the graph.
func verifyGraphInvariants(scenario string, uidToNode map[types.UID]*node, t *testing.T) {
	for myUID, node := range uidToNode {
		for dependentNode := range node.dependents {
			found := false
			for _, owner := range dependentNode.owners {
				if owner.UID == myUID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("scenario: %s: node %s has node %s as a dependent, but it's not present in the latter node's owners list", scenario, node.identity, dependentNode.identity)
			}
		}

		for _, owner := range node.owners {
			ownerNode, ok := uidToNode[owner.UID]
			if !ok {
				// It's possible that the owner node doesn't exist
				continue
			}
			if _, ok := ownerNode.dependents[node]; !ok {
				t.Errorf("node %s has node %s as an owner, but it's not present in the latter node's dependents list", node.identity, ownerNode.identity)
			}
		}
	}
}

func createEvent(eventType eventType, selfUID string, owners []string) event {
	var ownerReferences []api.OwnerReference
	for i := 0; i < len(owners); i++ {
		ownerReferences = append(ownerReferences, api.OwnerReference{UID: types.UID(owners[i])})
	}
	return event{
		eventType: eventType,
		obj: &api.Pod{
			ObjectMeta: api.ObjectMeta{
				UID:             types.UID(selfUID),
				OwnerReferences: ownerReferences,
			},
		},
	}
}

func TestProcessEvent(t *testing.T) {
	var testScenarios = []struct {
		name string
		// a series of events that will be supplied to the
		// Propagator.eventQueue.
		events []event
	}{
		{
			name: "test1",
			events: []event{
				createEvent(addEvent, "1", []string{}),
				createEvent(addEvent, "2", []string{"1"}),
				createEvent(addEvent, "3", []string{"1", "2"}),
			},
		},
		{
			name: "test2",
			events: []event{
				createEvent(addEvent, "1", []string{}),
				createEvent(addEvent, "2", []string{"1"}),
				createEvent(addEvent, "3", []string{"1", "2"}),
				createEvent(addEvent, "4", []string{"2"}),
				createEvent(deleteEvent, "2", []string{"doesn't matter"}),
			},
		},
		{
			name: "test3",
			events: []event{
				createEvent(addEvent, "1", []string{}),
				createEvent(addEvent, "2", []string{"1"}),
				createEvent(addEvent, "3", []string{"1", "2"}),
				createEvent(addEvent, "4", []string{"3"}),
				createEvent(updateEvent, "2", []string{"4"}),
			},
		},
		{
			name: "reverse test2",
			events: []event{
				createEvent(addEvent, "4", []string{"2"}),
				createEvent(addEvent, "3", []string{"1", "2"}),
				createEvent(addEvent, "2", []string{"1"}),
				createEvent(addEvent, "1", []string{}),
				createEvent(deleteEvent, "2", []string{"doesn't matter"}),
			},
		},
	}

	for _, scenario := range testScenarios {
		propagator := &Propagator{
			eventQueue: workqueue.New(),
			uidToNode: &concurrentUIDToNode{
				RWMutex:   &sync.RWMutex{},
				uidToNode: make(map[types.UID]*node),
			},
			gc: &GarbageCollector{
				dirtyQueue: workqueue.New(),
			},
		}
		for i := 0; i < len(scenario.events); i++ {
			propagator.eventQueue.Add(scenario.events[i])
			propagator.processEvent()
			verifyGraphInvariants(scenario.name, propagator.uidToNode.uidToNode, t)
		}
	}
}

// TestDependentsRace relies on golang's data race detector to check if there is
// data race among in the dependents field.
func TestDependentsRace(t *testing.T) {
	clientPool := dynamic.NewClientPool(&restclient.Config{}, dynamic.LegacyAPIPathResolverFunc)
	podResource := []unversioned.GroupVersionResource{{Version: "v1", Resource: "pods"}}
	gc, err := NewGarbageCollector(clientPool, podResource)
	if err != nil {
		t.Fatal(err)
	}

	const updates = 100
	owner := &node{dependentsLock: &sync.RWMutex{}, dependents: make(map[*node]struct{})}
	ownerUID := types.UID("owner")
	gc.propagator.uidToNode.Write(owner)
	go func() {
		for i := 0; i < updates; i++ {
			dependent := &node{}
			gc.propagator.addDependentToOwners(dependent, []metatypes.OwnerReference{{UID: ownerUID}})
			gc.propagator.removeDependentFromOwners(dependent, []metatypes.OwnerReference{{UID: ownerUID}})
		}
	}()
	go func() {
		gc.orphanQueue.Add(owner)
		for i := 0; i < updates; i++ {
			gc.orphanFinalizer()
		}
	}()
}

// test the list and watch functions correctly converts the ListOptions
func TestGCListWatcher(t *testing.T) {
	testHandler := &fakeActionHandler{}
	srv, clientConfig := testServerAndClientConfig(testHandler.ServeHTTP)
	defer srv.Close()
	clientPool := dynamic.NewClientPool(clientConfig, dynamic.LegacyAPIPathResolverFunc)
	podResource := unversioned.GroupVersionResource{Version: "v1", Resource: "pods"}
	client, err := clientPool.ClientForGroupVersion(podResource.GroupVersion())
	if err != nil {
		t.Fatal(err)
	}
	lw := gcListWatcher(client, podResource)
	lw.Watch(api.ListOptions{ResourceVersion: "1"})
	lw.List(api.ListOptions{ResourceVersion: "1"})
	if e, a := 2, len(testHandler.actions); e != a {
		t.Errorf("expect %d requests, got %d", e, a)
	}
	if e, a := "resourceVersion=1", testHandler.actions[0].query; e != a {
		t.Errorf("expect %s, got %s", e, a)
	}
	if e, a := "resourceVersion=1", testHandler.actions[1].query; e != a {
		t.Errorf("expect %s, got %s", e, a)
	}
}
