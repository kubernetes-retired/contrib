package kubernetes

import (
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kube_record "k8s.io/kubernetes/pkg/client/record"
	v1core "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"

	"github.com/golang/glog"
)

// CreateEventRecorder creates an event recorder to send custom events to Kubernetes to be recorded for targeted Kubernetes objects
func CreateEventRecorder(kubeClient kube_client.Interface) kube_record.EventRecorder {
	eventBroadcaster := kube_record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.Core().Events("")})
	return eventBroadcaster.NewRecorder(apiv1.EventSource{Component: "cluster-autoscaler"})
}
