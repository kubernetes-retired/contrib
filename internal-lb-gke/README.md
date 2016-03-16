# Internal Load-Balancer for GKE cluster

#### It is based on [GC Internal LB](https://cloud.google.com/solutions/internal-load-balancing-haproxy)

GKE Internal Load-Balancer is a tool that bootstraps a HAProxy VM, following the recommended [Google setup](https://cloud.google.com/solutions/internal-load-balancing-haproxy). However, this HAProxy VM now also watches your GKE cluster and updates the HAProxy configuration file and restarts HAProxy when it detects a new IP address.

How to use it:
---

- Run the command below:

```
$ git clone https://github.com/rimusz/gke-internal-lb
```
- Update `settings` file shown below with your GCE project and zone, and any other settings you want to change:

```
##############################################################################
# GC settings
# your project
PROJECT=_YOUR_PROJECT_
# your GKE cluster zone or which ever zone you want to put the internal LB VM
ZONE=_YOUR_ZONE_
#

# GKE cluster VM name without all those e.g -364478-node-sa5c
SERVERS=gke-cluster-1

# static IP for the internal LB VM
STATIC_IP=10.200.252.10

# VM type
MACHINE_TYPE=g1-small
##############################################################################
```
- Now run the script:

```
$ ./create_internal_lb.sh
```

What it does:
---
Running this script performs the following actions:

1. Checks for an existing setup and deletes it, along with all dependencies
2. Creates a temporary VM, and sets it up
3. Deletes the temporary VM keeping its boot disk
4. Creates the custom image from the boot disk
5. Creates a HAProxy instance template based on the custom image
6. Creates a managed instance group with the new VM


After that, you'll have an internal load balancer VM running HAProxy that forwards all received HTTP traffic to all GKE cluster nodes on port 80. You can configure the port number by editing the `get_vms_ip.tmpl` file.

This load balancer VM is watched by the Instance Group Manager. If the VM stops it gets restarted. Similarly, the HAProxy service running on the load balancer VM is set to always be running. So systemd restarts the service if it stops.

And here's the best bit: `/opt/bin/get_vms_ip` gets run by cron every two minutes. This script checks for cluster IP changes. If it detects any, it updates HAProxy configuration file as needed, and restarts HAProxy.

You can configure the cron frequency by editing the `create_internal_lb.sh` file.

#####Everytime time you run `create_internal_lb.sh` it checks for the existing HAProxy VM and if found it get's deleted and recreated again.



