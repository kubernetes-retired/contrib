package core

import (
	"testing"

	"k8s.io/contrib/cluster-autoscaler/config/dynamic"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"

	. "k8s.io/contrib/cluster-autoscaler/utils/test"

	"fmt"
	//"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/assert"
	"k8s.io/contrib/cluster-autoscaler/simulator"
	"k8s.io/contrib/cluster-autoscaler/utils/kubernetes"
	"time"
)

func TestNewAutoscalerStatic(t *testing.T) {
	fakeClient := &fake.Clientset{}
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, false, time.Now().Add(-3*time.Minute))
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Time{})
	p2 := BuildTestPod("p2", 800, 0)
	p2.Spec.NodeName = "n2"

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{*p2}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})
	kubeEventRecorder := kubernetes.CreateEventRecorder(fakeClient)
	opts := AutoscalerOptions{
		ConfigFetcherOptions: dynamic.ConfigFetcherOptions{
			ConfigMapName: "",
		},
	}
	predicateChecker := simulator.NewTestPredicateChecker()
	a := NewAutoscaler(opts, predicateChecker, fakeClient, kubeEventRecorder)
	assert.IsType(t, &StaticAutoscaler{}, a)
}

func TestNewAutoscalerDynamic(t *testing.T) {
	fakeClient := &fake.Clientset{}
	n1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(n1, false, time.Now().Add(-3*time.Minute))
	n2 := BuildTestNode("n2", 1000, 1000)
	SetNodeReadyState(n2, true, time.Time{})
	p2 := BuildTestPod("p2", 800, 0)
	p2.Spec.NodeName = "n2"

	fakeClient.Fake.AddReactor("list", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, &apiv1.PodList{Items: []apiv1.Pod{*p2}}, nil
	})
	fakeClient.Fake.AddReactor("get", "pods", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewNotFound(apiv1.Resource("pod"), "whatever")
	})
	fakeClient.Fake.AddReactor("get", "nodes", func(action core.Action) (bool, runtime.Object, error) {
		getAction := action.(core.GetAction)
		switch getAction.GetName() {
		case n1.Name:
			return true, n1, nil
		case n2.Name:
			return true, n2, nil
		}
		return true, nil, fmt.Errorf("Wrong node: %v", getAction.GetName())
	})
	kubeEventRecorder := kubernetes.CreateEventRecorder(fakeClient)
	opts := AutoscalerOptions{
		ConfigFetcherOptions: dynamic.ConfigFetcherOptions{
			ConfigMapName: "testconfigmap",
		},
	}
	predicateChecker := simulator.NewTestPredicateChecker()
	a := NewAutoscaler(opts, predicateChecker, fakeClient, kubeEventRecorder)
	assert.IsType(t, &DynamicAutoscaler{}, a)
}
