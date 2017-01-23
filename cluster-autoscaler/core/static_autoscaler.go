package core

import (
	"time"

	"k8s.io/contrib/cluster-autoscaler/metrics"
	kube_util "k8s.io/contrib/cluster-autoscaler/utils/kubernetes"

	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kube_record "k8s.io/kubernetes/pkg/client/record"

	"github.com/golang/glog"
)

// StaticAutoscaler is an autoscaler which has all the core functionality of a CA but without the reconfiguration feature
type StaticAutoscaler struct {
	// AutoscalingContext consists of validated settings and options for this autoscaler
	*AutoscalingContext
	readyNodeLister *kube_util.ReadyNodeLister
	scheduledPodLister *kube_util.ScheduledPodLister
	unschedulablePodLister  *kube_util.UnschedulablePodLister
	allNodeLister *kube_util.AllNodeLister
	kubeClient kube_client.Interface
	lastScaleUpTime time.Time
	lastScaleDownFailedTrial time.Time
	scaleDown *ScaleDown
}

// NewStaticAutoscaler creates an instance of Autoscaler filled with provided parameters
func NewStaticAutoscaler(opts AutoscalingOptions, kubeClient kube_client.Interface, kubeEventRecorder kube_record.EventRecorder) *StaticAutoscaler {
	autoscalingContext := NewAutoscalingContext(opts, kubeClient, kubeEventRecorder)

	scaleDown := NewScaleDown(autoscalingContext)
	stopchannel := make(chan struct{})
	unschedulablePodLister := kube_util.NewUnschedulablePodLister(kubeClient, stopchannel)
	scheduledPodLister := kube_util.NewScheduledPodLister(kubeClient, stopchannel)
	readyNodeLister := kube_util.NewReadyNodeLister(kubeClient, stopchannel)
	allNodeLister := kube_util.NewAllNodeLister(kubeClient, stopchannel)

	return &StaticAutoscaler{
		AutoscalingContext: autoscalingContext,
		readyNodeLister: readyNodeLister,
		unschedulablePodLister: unschedulablePodLister,
		scheduledPodLister: scheduledPodLister,
		allNodeLister: allNodeLister,
		lastScaleUpTime: time.Now(),
		lastScaleDownFailedTrial: time.Now(),
		scaleDown: scaleDown,
	}
}

// RunOnce iterates over node groups and scales them up/down if necessary
func (a *StaticAutoscaler) RunOnce(currentTime time.Time) {
	readyNodeLister := a.readyNodeLister
	kubeClient := a.kubeClient
	allNodeLister := a.allNodeLister
	unschedulablePodLister := a.unschedulablePodLister
	scheduledPodLister := a.scheduledPodLister
	scaleDown := a.scaleDown
	autoscalingContext := a.AutoscalingContext

	// CA can die at any time. Removing taints that might have been left from the previous run.
	if readyNodes, err := readyNodeLister.List(); err != nil {
		cleanToBeDeleted(readyNodes, kubeClient, a.Recorder)
	}

	readyNodes, err := readyNodeLister.List()
	if err != nil {
		glog.Errorf("Failed to list ready nodes: %v", err)
		return
	}
	if len(readyNodes) == 0 {
		glog.Errorf("No ready nodes in the cluster")
		return
	}

	allNodes, err := allNodeLister.List()
	if err != nil {
		glog.Errorf("Failed to list all nodes: %v", err)
		return
	}
	if len(allNodes) == 0 {
		glog.Errorf("No nodes in the cluster")
		return
	}

	a.ClusterStateRegistry.UpdateNodes(allNodes, currentTime)
	if !a.ClusterStateRegistry.IsClusterHealthy(time.Now()) {
		glog.Warningf("Cluster is not ready for autoscaling: %v", err)
		return
	}

	// Check if there are any nodes that failed to register in kuberentes
	// master.
	unregisteredNodes := a.ClusterStateRegistry.GetUnregisteredNodes()
	if len(unregisteredNodes) > 0 {
		glog.V(1).Infof("%d unregistered nodes present", len(unregisteredNodes))
		removedAny, err := removeOldUnregisteredNodes(unregisteredNodes, autoscalingContext, time.Now())
		// There was a problem with removing unregistered nodes. Retry in the next loop.
		if err != nil {
			if removedAny {
				glog.Warningf("Some unregistered nodes were removed, but got error: %v", err)
			} else {
				glog.Warningf("Failed to remove unregistered nodes: %v", err)

			}
			return
		}
		// Some nodes were removed. Let's skip this iteration, the next one should be better.
		if removedAny {
			glog.V(0).Infof("Some unregistered nodes were removed, skipping iteration")
			return
		}
	}

	// Check if there has been a constant difference between the number of nodes in k8s and
	// the number of nodes on the cloud provider side.
	// TODO: andrewskim - add protection for ready AWS nodes.
	fixedSomething, err := fixNodeGroupSize(autoscalingContext, time.Now())
	if err != nil {
		glog.Warningf("Failed to fix node group sizes: %v", err)
		return
	}
	if fixedSomething {
		glog.V(0).Infof("Some node group target size was fixed, skipping the iteration")
		return
	}

	allUnschedulablePods, err := unschedulablePodLister.List()
	if err != nil {
		glog.Errorf("Failed to list unscheduled pods: %v", err)
		return
	}

	allScheduled, err := scheduledPodLister.List()
	if err != nil {
		glog.Errorf("Failed to list scheduled pods: %v", err)
		return
	}

	// We need to reset all pods that have been marked as unschedulable not after
	// the newest node became available for the scheduler.
	allNodesAvailableTime := GetAllNodesAvailableTime(readyNodes)
	podsToReset, unschedulablePodsToHelp := SlicePodsByPodScheduledTime(allUnschedulablePods, allNodesAvailableTime)
	ResetPodScheduledCondition(a.AutoscalingContext.ClientSet, podsToReset)

	// We need to check whether pods marked as unschedulable are actually unschedulable.
	// This should prevent from adding unnecessary nodes. Example of such situation:
	// - CA and Scheduler has slightly different configuration
	// - Scheduler can't schedule a pod and marks it as unschedulable
	// - CA added a node which should help the pod
	// - Scheduler doesn't schedule the pod on the new node
	//   because according to it logic it doesn't fit there
	// - CA see the pod is still unschedulable, so it adds another node to help it
	//
	// With the check enabled the last point won't happen because CA will ignore a pod
	// which is supposed to schedule on an existing node.
	//
	// Without below check cluster might be unnecessary scaled up to the max allowed size
	// in the describe situation.
	schedulablePodsPresent := false
	if a.VerifyUnschedulablePods {
		newUnschedulablePodsToHelp := FilterOutSchedulable(unschedulablePodsToHelp, readyNodes, allScheduled,
			a.PredicateChecker)

		if len(newUnschedulablePodsToHelp) != len(unschedulablePodsToHelp) {
			glog.V(2).Info("Schedulable pods present")
			schedulablePodsPresent = true
		}
		unschedulablePodsToHelp = newUnschedulablePodsToHelp
	}

	if len(unschedulablePodsToHelp) == 0 {
		glog.V(1).Info("No unschedulable pods")
	} else if a.MaxNodesTotal > 0 && len(readyNodes) >= a.MaxNodesTotal {
		glog.V(1).Info("Max total nodes in cluster reached")
	} else {
		scaleUpStart := time.Now()
		metrics.UpdateLastTime("scaleup")
		scaledUp, err := ScaleUp(autoscalingContext, unschedulablePodsToHelp, readyNodes)

		metrics.UpdateDuration("scaleup", scaleUpStart)

		if err != nil {
			glog.Errorf("Failed to scale up: %v", err)
			return
		} else {
			if scaledUp {
				a.lastScaleUpTime = time.Now()
				// No scale down in this iteration.
				return
			}
		}
	}

	if a.ScaleDownEnabled {
		unneededStart := time.Now()

		// In dry run only utilization is updated
		calculateUnneededOnly := a.lastScaleUpTime.Add(a.ScaleDownDelay).After(time.Now()) ||
			a.lastScaleDownFailedTrial.Add(a.ScaleDownTrialInterval).After(time.Now()) ||
			schedulablePodsPresent

		glog.V(4).Infof("Scale down status: unneededOnly=%v lastScaleUpTime=%s "+
			"lastScaleDownFailedTrail=%s schedulablePodsPresent=%v", calculateUnneededOnly,
			a.lastScaleUpTime, a.lastScaleDownFailedTrial, schedulablePodsPresent)

		metrics.UpdateLastTime("findUnneeded")
		glog.V(4).Infof("Calculating unneeded nodes")

		scaleDown.CleanUp(time.Now())
		err := scaleDown.UpdateUnneededNodes(allNodes, allScheduled, time.Now())
		if err != nil {
			glog.Warningf("Failed to scale down: %v", err)
			return
		}

		metrics.UpdateDuration("findUnneeded", unneededStart)

		for key, val := range scaleDown.unneededNodes {
			if glog.V(4) {
				glog.V(4).Infof("%s is unneeded since %s duration %s", key, val.String(), time.Now().Sub(val).String())
			}
		}

		if !calculateUnneededOnly {
			glog.V(4).Infof("Starting scale down")

			scaleDownStart := time.Now()
			metrics.UpdateLastTime("scaledown")
			result, err := scaleDown.TryToScaleDown(allNodes, allScheduled)
			metrics.UpdateDuration("scaledown", scaleDownStart)

			// TODO: revisit result handling
			if err != nil {
				glog.Errorf("Failed to scale down: %v", err)
			} else {
				if result == ScaleDownError || result == ScaleDownNoNodeDeleted {
					a.lastScaleDownFailedTrial = time.Now()
				}
			}
		}
	}
}
