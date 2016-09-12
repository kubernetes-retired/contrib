# Cluster Autoscaler

# Introduction

Cluster Autoscaler is a tool that automatically adjusts the size of the Kubernetes cluster when:
* there is a pod that doesn’t have enough space to run in the cluster
* some nodes in the cluster are so underutilized that they can be deleted and their pods will 
be easily placed on some other, existing nodes.  

# Deployment

Cluster Autoscaler runs on the Kubernetes master node (at least in the default setup on GCE and GKE). 
It is possible to run customized Cluster Autoscaler inside of the cluster but then extra care needs
to be taken to ensure that Cluster Autoscaler is up and running. User can put it into kube-system
namespace (Cluster Autoscaler doesn't scale down node with non-manifest based kube-system pods running
on them) and mark with `scheduler.alpha.kubernetes.io/critical-pod` annotation (so that the rescheduler, 
if enabled, will kill other pods to make space for it to run). 

Right now it is possible to run Cluster Autoscaler on:
* GCE http://kubernetes.io/docs/admin/cluster-management/#cluster-autoscaling
* GKE https://cloud.google.com/container-engine/docs/cluster-autoscaler
* AWS https://github.com/kubernetes/contrib/blob/master/cluster-autoscaler/cloudprovider/aws/README.md

# Scale Up

Scale up creates a watch on the api server looking for all pods. Every 10 seconds (configurable)
it checks for any unschedulable pods. Unschedulable pods are recognized by their PodCondition. 
Whenever a kubernetes scheduler fails to find a place to run a pod it sets "schedulable" 
PodCondition to false and reason to "unschedulable".  If there are any items on the unschedulable 
lists Cluster Autoscaler tries to find a new place to run them. 

It is assumed that the underlying cluster is run on top of some kind of node groups.
Inside a node group all machines have identical capacity and have the same set of assigned labels. 
Thus increasing a size of a node pool will bring a couple of new machines that will be similar 
to these that are already in the cluster - they will just not have the user-created pods (but 
will have all pods run from the node manifest or daemon sets). 

Based on the above assumption Cluster Autoscaler creates template nodes for each of the 
node groups and checks if any of the unschedulable pods would fit to a brand new node, if created.
If there are multiple node groups that, if increased, would help with getting some pods running, 
one of them is selected at random. 

# Scale Down

Every 10 seconds (configurable) Cluster Autoscaler checks which nodes are not so needed and can 
be removed. A node is considered not needed when:

* The sum of cpu and memory requests of all pod running on this node is smaller than 50% of node
capacity.

* All pods running on the node (except these that run on all nodes by default like manifest-run pods
or pods created by daemonsets) can be moved to some other nodes. Stand-alone pods which are not
under control of a deployment, replica set, replication controller or job would not be recreated
if the node is deleted so they make a node needed, even if its utilization is low. While 
checking this condition the new locations of all pods are memorized.

* There are no kube-system pods on the node (except these that run on all nodes by default like 
manifest-run pods or pods created by daemonsets).

If a node is not needed for more than 10 min (configurable) then it can be deleted. Cluster Autoscaler
deletes one node at a time to reduce the risk of creating new unschedulable pods. The next node 
can be deleted when it is also not needed for more than 10 min. It may happen just after
the previous node is fully deleted or after some longer time.

As mentioned above we delete node A only if for 10 minutes it was possible to move all of its 
relevant pods elsewhere, let's say to nodes X and Z. After deletion of A the free capacity of X and Z
is likely to be decreased. So if, during the simulations, Cluster Autoscaler placed pods from node
B on nodes X and Y then it is possible that if Cluster Autoscaler tried to simulate the deletion of both
A and B at the same time some other nodes than X,Y and Z would be involved in pods relocation.
Thus Cluster Autoscaler may be forced to measure if node B is not needed from the beginning. 
But if there is some other node C that, on deletion, would move its pods to node W then the removal
of node A doesn't (hopefully) affect it too much. Such node C can be removed immediately after A.

# When scaling is executed

A strict requirement for performing any scale operations is that the size of a node group,
measured on the cloud provider side, matches the number of nodes in Kubernetes that belong to this 
node group. If this condition is not met then all scaling operations are postponed until it is 
fulfilled. 
Also, any scale down will happen only after at least 10 min after the last scale up.