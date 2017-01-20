/*
Copyright 2015 The Kubernetes Authors.

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

package cache

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/v1"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
)

func TestStoreToNodeLister(t *testing.T) {
	store := NewStore(MetaNamespaceKeyFunc)
	ids := sets.NewString("foo", "bar", "baz")
	for id := range ids {
		store.Add(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: id}})
	}
	sml := StoreToNodeLister{store}

	gotNodes, err := sml.List()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	got := make([]string, len(gotNodes.Items))
	for ix := range gotNodes.Items {
		got[ix] = gotNodes.Items[ix].Name
	}
	if !ids.HasAll(got...) || len(got) != len(ids) {
		t.Errorf("Expected %v, got %v", ids, got)
	}
}

func TestStoreToNodeConditionLister(t *testing.T) {
	store := NewStore(MetaNamespaceKeyFunc)
	nodes := []*v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
					{
						Type:   v1.NodeOutOfDisk,
						Status: v1.ConditionFalse,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "bar"},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeOutOfDisk,
						Status: v1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "baz"},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionFalse,
					},
					{
						Type:   v1.NodeOutOfDisk,
						Status: v1.ConditionUnknown,
					},
				},
			},
		},
	}
	for _, n := range nodes {
		store.Add(n)
	}

	predicate := func(node *v1.Node) bool {
		for _, cond := range node.Status.Conditions {
			if cond.Type == v1.NodeOutOfDisk && cond.Status == v1.ConditionTrue {
				return false
			}
		}
		return true
	}

	snl := StoreToNodeLister{store}
	sncl := snl.NodeCondition(predicate)

	want := sets.NewString("foo", "baz")
	gotNodes, err := sncl.List()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	got := make([]string, len(gotNodes))
	for ix := range gotNodes {
		got[ix] = gotNodes[ix].Name
	}
	if !want.HasAll(got...) || len(got) != len(want) {
		t.Errorf("Expected %v, got %v", want, got)
	}
}

func TestStoreToReplicationControllerLister(t *testing.T) {
	testCases := []struct {
		description              string
		inRCs                    []*v1.ReplicationController
		list                     func(StoreToReplicationControllerLister) ([]*v1.ReplicationController, error)
		outRCNames               sets.String
		expectErr                bool
		onlyIfIndexedByNamespace bool
	}{
		{
			description: "Verify we can search all namespaces",
			inRCs: []*v1.ReplicationController{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "hmm", Namespace: "hmm"},
				},
			},
			list: func(lister StoreToReplicationControllerLister) ([]*v1.ReplicationController, error) {
				return lister.ReplicationControllers(v1.NamespaceAll).List(labels.Set{}.AsSelectorPreValidated())
			},
			outRCNames: sets.NewString("hmm", "foo"),
		},
		{
			description: "Verify we can search a specific namespace",
			inRCs: []*v1.ReplicationController{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "hmm", Namespace: "hmm"},
				},
			},
			list: func(lister StoreToReplicationControllerLister) ([]*v1.ReplicationController, error) {
				return lister.ReplicationControllers("hmm").List(labels.Set{}.AsSelectorPreValidated())
			},
			outRCNames: sets.NewString("hmm"),
		},
		{
			description: "Basic listing with all labels and no selectors",
			inRCs: []*v1.ReplicationController{
				{ObjectMeta: metav1.ObjectMeta{Name: "basic"}},
			},
			list: func(lister StoreToReplicationControllerLister) ([]*v1.ReplicationController, error) {
				return lister.List(labels.Everything())
			},
			outRCNames: sets.NewString("basic"),
		},
		{
			description: "No pod labels",
			inRCs: []*v1.ReplicationController{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: "ns"},
					Spec: v1.ReplicationControllerSpec{
						Selector: map[string]string{"foo": "baz"},
					},
				},
			},
			list: func(lister StoreToReplicationControllerLister) ([]*v1.ReplicationController, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns"},
				}
				return lister.GetPodControllers(pod)
			},
			outRCNames: sets.NewString(),
			expectErr:  true,
		},
		{
			description: "No RC selectors",
			inRCs: []*v1.ReplicationController{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: "ns"},
				},
			},
			list: func(lister StoreToReplicationControllerLister) ([]*v1.ReplicationController, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns",
						Labels:    map[string]string{"foo": "bar"},
					},
				}
				return lister.GetPodControllers(pod)
			},
			outRCNames: sets.NewString(),
			expectErr:  true,
		},
		{
			description: "Matching labels to selectors and namespace",
			inRCs: []*v1.ReplicationController{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: v1.ReplicationControllerSpec{
						Selector: map[string]string{"foo": "bar"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: "ns"},
					Spec: v1.ReplicationControllerSpec{
						Selector: map[string]string{"foo": "bar"},
					},
				},
			},
			list: func(lister StoreToReplicationControllerLister) ([]*v1.ReplicationController, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Labels:    map[string]string{"foo": "bar"},
						Namespace: "ns",
					},
				}
				return lister.GetPodControllers(pod)
			},
			outRCNames:               sets.NewString("bar"),
			onlyIfIndexedByNamespace: true,
		},
	}
	for _, c := range testCases {
		for _, withIndex := range []bool{true, false} {
			if c.onlyIfIndexedByNamespace && !withIndex {
				continue
			}
			var store Indexer
			if withIndex {
				store = NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
			} else {
				store = NewIndexer(MetaNamespaceKeyFunc, Indexers{})
			}

			for _, r := range c.inRCs {
				store.Add(r)
			}

			gotControllers, err := c.list(StoreToReplicationControllerLister{store})
			if err != nil && c.expectErr {
				continue
			} else if c.expectErr {
				t.Errorf("(%q, withIndex=%v) Expected error, got none", c.description, withIndex)
				continue
			} else if err != nil {
				t.Errorf("(%q, withIndex=%v) Unexpected error %#v", c.description, withIndex, err)
				continue
			}
			gotNames := make([]string, len(gotControllers))
			for ix := range gotControllers {
				gotNames[ix] = gotControllers[ix].Name
			}
			if !c.outRCNames.HasAll(gotNames...) || len(gotNames) != len(c.outRCNames) {
				t.Errorf("(%q, withIndex=%v) Unexpected got controllers %+v expected %+v", c.description, withIndex, gotNames, c.outRCNames)
			}
		}
	}
}

func TestStoreToReplicaSetLister(t *testing.T) {
	store := NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
	lister := StoreToReplicaSetLister{store}
	testCases := []struct {
		inRSs      []*extensions.ReplicaSet
		list       func() ([]*extensions.ReplicaSet, error)
		outRSNames sets.String
		expectErr  bool
	}{
		// Basic listing with all labels and no selectors
		{
			inRSs: []*extensions.ReplicaSet{
				{ObjectMeta: metav1.ObjectMeta{Name: "basic"}},
			},
			list: func() ([]*extensions.ReplicaSet, error) {
				return lister.List(labels.Everything())
			},
			outRSNames: sets.NewString("basic"),
		},
		// No pod labels
		{
			inRSs: []*extensions.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: "ns"},
					Spec: extensions.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "baz"}},
					},
				},
			},
			list: func() ([]*extensions.ReplicaSet, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns"},
				}
				return lister.GetPodReplicaSets(pod)
			},
			outRSNames: sets.NewString(),
			expectErr:  true,
		},
		// No ReplicaSet selectors
		{
			inRSs: []*extensions.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: "ns"},
				},
			},
			list: func() ([]*extensions.ReplicaSet, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns",
						Labels:    map[string]string{"foo": "bar"},
					},
				}
				return lister.GetPodReplicaSets(pod)
			},
			outRSNames: sets.NewString(),
			expectErr:  true,
		},
		// Matching labels to selectors and namespace
		{
			inRSs: []*extensions.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: extensions.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: "ns"},
					Spec: extensions.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					},
				},
			},
			list: func() ([]*extensions.ReplicaSet, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Labels:    map[string]string{"foo": "bar"},
						Namespace: "ns",
					},
				}
				return lister.GetPodReplicaSets(pod)
			},
			outRSNames: sets.NewString("bar"),
		},
	}
	for _, c := range testCases {
		for _, r := range c.inRSs {
			store.Add(r)
		}

		gotRSs, err := c.list()
		if err != nil && c.expectErr {
			continue
		} else if c.expectErr {
			t.Error("Expected error, got none")
			continue
		} else if err != nil {
			t.Errorf("Unexpected error %#v", err)
			continue
		}
		gotNames := make([]string, len(gotRSs))
		for ix := range gotRSs {
			gotNames[ix] = gotRSs[ix].Name
		}
		if !c.outRSNames.HasAll(gotNames...) || len(gotNames) != len(c.outRSNames) {
			t.Errorf("Unexpected got ReplicaSets %+v expected %+v", gotNames, c.outRSNames)
		}
	}
}

func TestStoreToDaemonSetLister(t *testing.T) {
	store := NewStore(MetaNamespaceKeyFunc)
	lister := StoreToDaemonSetLister{store}
	testCases := []struct {
		inDSs             []*extensions.DaemonSet
		list              func() ([]extensions.DaemonSet, error)
		outDaemonSetNames sets.String
		expectErr         bool
	}{
		// Basic listing
		{
			inDSs: []*extensions.DaemonSet{
				{ObjectMeta: metav1.ObjectMeta{Name: "basic"}},
			},
			list: func() ([]extensions.DaemonSet, error) {
				list, err := lister.List()
				return list.Items, err
			},
			outDaemonSetNames: sets.NewString("basic"),
		},
		// Listing multiple daemon sets
		{
			inDSs: []*extensions.DaemonSet{
				{ObjectMeta: metav1.ObjectMeta{Name: "basic"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "complex"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "complex2"}},
			},
			list: func() ([]extensions.DaemonSet, error) {
				list, err := lister.List()
				return list.Items, err
			},
			outDaemonSetNames: sets.NewString("basic", "complex", "complex2"),
		},
		// No pod labels
		{
			inDSs: []*extensions.DaemonSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: "ns"},
					Spec: extensions.DaemonSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "baz"}},
					},
				},
			},
			list: func() ([]extensions.DaemonSet, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns"},
				}
				return lister.GetPodDaemonSets(pod)
			},
			outDaemonSetNames: sets.NewString(),
			expectErr:         true,
		},
		// No DS selectors
		{
			inDSs: []*extensions.DaemonSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "basic", Namespace: "ns"},
				},
			},
			list: func() ([]extensions.DaemonSet, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns",
						Labels:    map[string]string{"foo": "bar"},
					},
				}
				return lister.GetPodDaemonSets(pod)
			},
			outDaemonSetNames: sets.NewString(),
			expectErr:         true,
		},
		// Matching labels to selectors and namespace
		{
			inDSs: []*extensions.DaemonSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo"},
					Spec: extensions.DaemonSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: "ns"},
					Spec: extensions.DaemonSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					},
				},
			},
			list: func() ([]extensions.DaemonSet, error) {
				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Labels:    map[string]string{"foo": "bar"},
						Namespace: "ns",
					},
				}
				return lister.GetPodDaemonSets(pod)
			},
			outDaemonSetNames: sets.NewString("bar"),
		},
	}
	for _, c := range testCases {
		for _, r := range c.inDSs {
			store.Add(r)
		}

		daemonSets, err := c.list()
		if err != nil && c.expectErr {
			continue
		} else if c.expectErr {
			t.Error("Expected error, got none")
			continue
		} else if err != nil {
			t.Errorf("Unexpected error %#v", err)
			continue
		}
		daemonSetNames := make([]string, len(daemonSets))
		for ix := range daemonSets {
			daemonSetNames[ix] = daemonSets[ix].Name
		}
		if !c.outDaemonSetNames.HasAll(daemonSetNames...) || len(daemonSetNames) != len(c.outDaemonSetNames) {
			t.Errorf("Unexpected got controllers %+v expected %+v", daemonSetNames, c.outDaemonSetNames)
		}
	}
}

func TestStoreToPodLister(t *testing.T) {
	// We test with and without a namespace index, because StoreToPodLister has
	// special logic to work on namespaces even when no namespace index is
	// present.
	stores := []Indexer{
		NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc}),
		NewIndexer(MetaNamespaceKeyFunc, Indexers{}),
	}
	for _, store := range stores {
		ids := []string{"foo", "bar", "baz"}
		for _, id := range ids {
			store.Add(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "other",
					Name:      id,
					Labels:    map[string]string{"name": id},
				},
			})
		}
		store.Add(&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "quux",
				Namespace: v1.NamespaceDefault,
				Labels:    map[string]string{"name": "quux"},
			},
		})
		spl := StoreToPodLister{store}

		// Verify that we can always look up by Namespace.
		defaultPods, err := spl.Pods(v1.NamespaceDefault).List(labels.Set{}.AsSelectorPreValidated())
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		} else if e, a := 1, len(defaultPods); e != a {
			t.Errorf("Expected %v, got %v", e, a)
		} else if e, a := "quux", defaultPods[0].Name; e != a {
			t.Errorf("Expected %v, got %v", e, a)
		}

		for _, id := range ids {
			got, err := spl.List(labels.Set{"name": id}.AsSelectorPreValidated())
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				continue
			}
			if e, a := 1, len(got); e != a {
				t.Errorf("Expected %v, got %v", e, a)
				continue
			}
			if e, a := id, got[0].Name; e != a {
				t.Errorf("Expected %v, got %v", e, a)
				continue
			}

			_, err = spl.Pods("other").Get(id)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}

		if _, err := spl.Pods("").Get("qux"); !apierrors.IsNotFound(err) {
			t.Error("Unexpected pod exists")
		}
	}
}

func TestStoreToServiceLister(t *testing.T) {
	store := NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
	store.Add(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{},
		},
	})
	store.Add(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})
	ssl := StoreToServiceLister{store}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "foopod",
			Labels: map[string]string{"role": "foo"},
		},
	}

	services, err := ssl.GetPodServices(pod)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %v", len(services))
	}
	if e, a := "foo", services[0].Name; e != a {
		t.Errorf("Expected service %q, got %q", e, a)
	}
}
