# kube-keepalived-vip
Kubernetes Virtual IP address/es using [keepalived](http://www.keepalived.org)

AKA "how to set up virtual IP addresses in kubernetes using [IPVS - The Linux Virtual Server Project](http://www.linuxvirtualserver.org/software/ipvs.html)".


## Overview

kubernetes v1.6 offers 3 ways to expose a service:

1. **L4 LoadBalancer**: [Available only on cloud providers](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/) such as GCE and AWS
1. **Service via NodePort**: The [NodePort directive](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport) allocates a port on every worker node, which proxy the traffic to the respective Pod.
1. **L7 Ingress**: The [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) is a dedicated loadbalancer (eg. nginx, HAProxy, traefik, vulcand) that redirects incoming HTTP/HTTPS traffic to the respective endpoints

If this works, why do we need _keepalived_?

```
                                                  ___________________
                                                 |                   |
                                           |-----| Host IP: 10.4.0.3 |
                                           |     |___________________|
                                           |
                                           |      ___________________
                                           |     |                   |
Public ----(example.com = 10.4.0.3/4/5)----|-----| Host IP: 10.4.0.4 |
                                           |     |___________________|
                                           |
                                           |      ___________________
                                           |     |                   |
                                           |-----| Host IP: 10.4.0.5 |
                                                 |___________________|
```

Let's assume that Ingress-es are run on the 3 kubernetes worker nodes above `10.4.0.x`, which are exposed to the public to load-balance incoming traffic. DNS Round Robin (RR) is applied to `example.com` to rotate between the 3 nodes. If `10.4.0.3` goes down, one-third of the traffic to `example.com` is still directed to the downed node (due to DNS RR). The sysadmin has to step in and delist the faulty node from `example.com`. Since there will be intermittent downtime until the sysadmin intervenes, this isn't true High Availability (HA).

Here is where IPVS can help. 

The idea is to expose a Virtual IP (VIP) address per service, outside of the kubernetes cluster. _keepalived_ then uses VRRP to sync this "mapping" in the local network. With 2 or more instance of the pod running in the cluster is possible to provide HA using a single VIP address.

##### What is the difference between _keepalived_ and [service-loadbalancer](https://github.com/kubernetes/contrib/tree/master/service-loadbalancer) or [nginx-alpha](https://github.com/kubernetes/contrib/tree/master/Ingress/controllers/nginx-alpha)?

_keepalived_ should be considered a complement to, and not a replacement for HAProxy or nginx. The goal is to provide robust HA, such that no downtime is experienced if one or more nodes go offline. To be exact, _keepalived_ ensures that the VIP, which exposes a service-loadbalancer or an Ingress, is always owned by a live node. The DNS record will simply point to this single VIP (ie. sans RR) and the failover will be handled entirely by _keepalived_.

```
                                               ___________________
                                              |                   |
                                              | VIP: 10.4.0.50    |
                                        |-----| Host IP: 10.4.0.3 |
                                        |     | Role: Master      |
                                        |     |___________________|
                                        |
                                        |      ___________________
                                        |     |                   |
                                        |     | VIP: Unassigned   |
Public ----(example.com = 10.4.0.50)----|-----| Host IP: 10.4.0.3 |
                                        |     | Role: Slave       |
                                        |     |___________________|
                                        |
                                        |      ___________________
                                        |     |                   |
                                        |     | VIP: Unassigned   |
                                        |-----| Host IP: 10.4.0.3 |
                                              | Role: Slave       |
                                              |___________________|
```

In the above diagram, one node assumes the role of a Master (negotiated via VRRP), and assumes the VIP. `example.com` points only to the shared VIP `10.4.0.50`, instead of the 3 nodes. If `10.4.0.3` is taken offline, the surviving hosts elect a new master to assume the VIP. This model of HA ensures that the VIP can be reached at all times.

## Requirements

The only requirement is for [DaemonSets](https://github.com/kubernetes/kubernetes/blob/master/docs/design/daemon.md) to be enabled is the only requirement. Check this [guide](https://github.com/kubernetes/kubernetes/blob/master/docs/api.md#enabling-resources-in-the-extensions-group) to include the `kube-apiserver` flags for this to work.


## Configuration

To expose one or more services use the flag `services-configmap`. The format of the data is: `external IP -> namespace/serviceName`. Optionally it is possible to specify forwarding method using `:` after the service name. The valid options are `NAT` and `DR`. For instance `external IP -> namespace/serviceName:DR`. By default, if the method is not specified it will use NAT.

This IP must be routable within the LAN and must be available. By default the IP address of the pods is used to route the traffic. This means that is one pod dies or a new one is created by a scale event the keepalived configuration file will be updated and reloaded.

## Example

### Launch the sample app "echoheaders"
First, we create a new ReplicationController and a Service for a sample app.
```
$ kubectl create -f examples/echoheaders.yaml
replicationcontroller "echoheaders" created
You have exposed your service on an external port on all nodes in your
cluster.  If you want to expose this service to the external internet, you may
need to set up firewall rules for the service port(s) (tcp:30302) to serve traffic.

See http://releases.k8s.io/HEAD/docs/user-guide/services-firewalls.md for more details.
service "echoheaders" created
```

### (Optional) Install the RBAC policies
If you enabled [RBAC](https://kubernetes.io/docs/admin/authorization/) in your cluster (ie. `kube-apiserver` runs with the `--authorization-mode=RBAC` flag), please follow this section so that _keepalived_ can properly query the cluster's API endpoints.

Create a service account so that _keepalived_ can authenticate with `kube-apiserver`.
```
kubectl create sa kube-keepalived-vip
```

Configure the DaemonSet in `vip-daemonset.yaml` to use the ServiceAccount. Add the `serviceAccount` to the file as shown:
```
    spec:
      hostNetwork: true
      serviceAccount: kube-keepalived-vip
      containers:
        - image: gcr.io/google_containers/kube-keepalived-vip:0.9
```

Configure its ClusterRole. _keepalived_ needs to read the pods, nodes, endpoints and services.
```
echo 'apiVersion: rbac.authorization.k8s.io/v1alpha1
kind: ClusterRole
metadata:
  name: kube-keepalived-vip
rules:
- apiGroups: [""]
  resources:
  - pods
  - nodes
  - endpoints
  - services
  - configmaps
  verbs: ["get", "list", "watch"]' | kubectl create -f -
```

Configure its ClusterRoleBinding. This binds the above ClusterRole to the `kube-keepalived-vip` ServiceAccount.
```
apiVersion: rbac.authorization.k8s.io/v1alpha1
kind: ClusterRoleBinding
metadata:
  name: kube-keepalived-vip
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kube-keepalived-vip
subjects:
- kind: ServiceAccount
  name: kube-keepalived-vip
  namespace: default
```

### Load the _keepalived_ DaemonSet
Next add the required annotation to expose the service using a local IP

```
$ echo "apiVersion: v1
kind: ConfigMap
metadata:
  name: vip-configmap
data:
  10.4.0.50: default/echoheaders" | kubectl create -f -
```

You can also set multiple services on a single ip, for example

```
data:
  10.4.0.50: |-
  default/echoheaders
  default/someanotherservice
```

But in this case you need to ensure that there are no any overlaps with services ports

Now the creation of the daemonset
```
$ kubectl create -f vip-daemonset.yaml
daemonset "kube-keepalived-vip" created
$ kubectl get daemonset
NAME                  CONTAINER(S)          IMAGE(S)                         SELECTOR                        NODE-SELECTOR
kube-keepalived-vip   kube-keepalived-vip   gcr.io/google_containers/kube-keepalived-vip:0.9   name in (kube-keepalived-vip)   type=worker
```

**Note: the DaemonSet yaml file contains a node selector. This is not required, is just an example to show how is possible to limit the nodes where keepalived can run**

### Debug the deployment
To verify if everything is working we should check if a `kube-keepalived-vip` pod is in each node of the cluster

Check the labels of the nodes.
```
$ kubectl get nodes
NAME       LABELS                                        STATUS    AGE
10.4.0.3   kubernetes.io/hostname=10.4.0.3,type=worker   Ready     1d
10.4.0.4   kubernetes.io/hostname=10.4.0.4,type=worker   Ready     1d
10.4.0.5   kubernetes.io/hostname=10.4.0.5,type=worker   Ready     1d
```

Check that there's a pod running on each node
```
$ kubectl get pods
NAME                        READY     STATUS    RESTARTS   AGE
echoheaders-co4g4           1/1       Running   0          5m
kube-keepalived-vip-a90bt   1/1       Running   0          53s
kube-keepalived-vip-g3nku   1/1       Running   0          52s
kube-keepalived-vip-gd18l   1/1       Running   0          54s
```

_keepalived_'s logs should look like this if no error was encountered.
```
$ kubectl logs kube-keepalived-vip-a90bt
I0410 14:24:45.860119       1 keepalived.go:161] cleaning ipvs configuration
I0410 14:24:45.873095       1 main.go:109] starting LVS configuration
I0410 14:24:45.894664       1 main.go:119] starting keepalived to announce VIPs
Starting Healthcheck child process, pid=17
Starting VRRP child process, pid=18
Initializing ipvs 2.6
Registering Kernel netlink reflector
Registering Kernel netlink reflector
Registering Kernel netlink command channel
Registering gratuitous ARP shared channel
Registering Kernel netlink command channel
Using LinkWatch kernel netlink reflector...
Using LinkWatch kernel netlink reflector...
I0410 14:24:56.017590       1 keepalived.go:151] reloading keepalived
Got SIGHUP, reloading checker configuration
Registering Kernel netlink reflector
Initializing ipvs 2.6
Registering Kernel netlink command channel
Registering gratuitous ARP shared channel
Registering Kernel netlink reflector
Opening file '/etc/keepalived/keepalived.conf'.
Registering Kernel netlink command channel
Opening file '/etc/keepalived/keepalived.conf'.
Using LinkWatch kernel netlink reflector...
VRRP_Instance(vips) Entering BACKUP STATE
Using LinkWatch kernel netlink reflector...
Activating healthchecker for service [10.2.68.5]:8080
VRRP_Instance(vips) Transition to MASTER STATE
VRRP_Instance(vips) Entering MASTER STATE
VRRP_Instance(vips) using locally configured advertisement interval (1000 milli-sec)
```

_keepalived_'s configuration is empty at the start. It should automatically be updated to reflect the current setup.
```
$ kubectl exec kube-keepalived-vip-a90bt cat /etc/keepalived/keepalived.conf

global_defs {
  vrrp_version 3
  vrrp_iptables KUBE-KEEPALIVED-VIP
}

vrrp_instance vips {
  state BACKUP
  interface eth1
  virtual_router_id 50
  priority 100
  nopreempt
  advert_int 1

  track_interface {
    eth1
  }



  virtual_ipaddress {
    172.17.4.90
  }
}


# Service: default/echoheaders
virtual_server 10.4.0.50 80 {
  delay_loop 5
  lvs_sched wlc
  lvs_method NAT
  persistence_timeout 1800
  protocol TCP


  real_server 10.2.68.5 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

}

```

Test that the app can be reached via the VIP `10.4.0.50`.
```
$ curl -v 10.4.0.50
* Rebuilt URL to: 10.4.0.50/
*   Trying 10.4.0.50...
* Connected to 10.4.0.50 (10.4.0.50) port 80 (#0)
> GET / HTTP/1.1
> Host: 10.4.0.50
> User-Agent: curl/7.43.0
> Accept: */*
>
* HTTP 1.0, assume close after body
< HTTP/1.0 200 OK
< Server: BaseHTTP/0.6 Python/3.5.0
< Date: Wed, 30 Dec 2015 19:52:39 GMT
<
CLIENT VALUES:
client_address=('10.4.0.148', 52178) (10.4.0.148)
command=GET
path=/
real path=/
query=
request_version=HTTP/1.1

SERVER VALUES:
server_version=BaseHTTP/0.6
sys_version=Python/3.5.0
protocol_version=HTTP/1.0

HEADERS RECEIVED:
Accept=*/*
Host=10.4.0.50
User-Agent=curl/7.43.0
* Closing connection 0

```

Scaling the replication controller should automatically update and reload keepalived.

```
$ kubectl scale --replicas=5 replicationcontroller echoheaders
replicationcontroller "echoheaders" scaled
```

The latest config should reflect something similar to this after scaling up the app.
```
$ kubectl exec kube-keepalived-vip-a90bt cat /etc/keepalived/keepalived.conf

global_defs {
  vrrp_version 3
  vrrp_iptables KUBE-KEEPALIVED-VIP
}

vrrp_instance vips {
  state BACKUP
  interface eth1
  virtual_router_id 50
  priority 100
  nopreempt
  advert_int 1

  track_interface {
    eth1
  }



  virtual_ipaddress {
    172.17.4.90
  }
}


# Service: default/echoheaders
virtual_server 10.4.0.50 80 {
  delay_loop 5
  lvs_sched wlc
  lvs_method NAT
  persistence_timeout 1800
  protocol TCP


  real_server 10.2.68.5 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.68.6 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.68.7 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.68.8 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.68.9 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

}
```

## Related projects

- https://github.com/kobolog/gorb
- https://github.com/qmsk/clusterf
- https://github.com/osrg/gobgp
