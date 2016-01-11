# kube-keepalived-vip
Kubernetes Virtual IP address/es using [keepalived](http://www.keepalived.org)

AKA "how to set up virtual IP addresses in kubernetes using [IPVS - The Linux Virtual Server Project](http://www.linuxvirtualserver.org/software/ipvs.html)".

## Disclaimer:
- This is a **work in progress**.

## Overview

There are 2 ways to expose a service in the current kubernetes service model:

- Create a cloud load balancer.
- Allocate a port (the same port) on every node in your cluster and proxy traffic through that port to the endpoints.

This just works. What's the issue then? 

The issue is that it does not provide High Availability because beforehand is required to know the IP addresss of the node where is running and in case of a failure the pod can be be moved to a different node. Here is where ipvs could help. 
The idea is to define an IP address per service to expose it outside the Kubernetes cluster and use vrrp to announce this "mapping" in the local network.
With 2 or more instance of the pod running in the cluster is possible to provide high availabity using a single IP address.

##### What is the difference between this and [service-loadbalancer](https://github.com/kubernetes/contrib/tree/master/service-loadbalancer) or [nginx-alpha](https://github.com/kubernetes/contrib/tree/master/Ingress/controllers/nginx-alpha) to expose one or more services?

This should be considered a complement, not a replacement for HAProxy or nginx. The goal using keepalived is to provide high availability and to bring certainty about how an exposed service can be reached (beforehand we know the ip address independently of the node where is running). For instance keepalived can use used to expose the service-loadbalancer or nginx ingress controller in the LAN using one IP address.



## Requirements

[Daemonsets](https://github.com/kubernetes/kubernetes/blob/master/docs/design/daemon.md) enabled is the only requirement. Check this [guide](https://github.com/kubernetes/kubernetes/blob/master/docs/api.md#enabling-resources-in-the-extensions-group) with the required flags in kube-apiserver.



## Configuration

To expose a service add the annotation `k8s.io/public-vip` in the service with the IP address to be use. This IP must be routable inside the LAN and must be available.
By default the IP address of the pods are used to route the traffic. This means that is one pod dies or a new one is created by a scale event the keepalived configuration file will be updated and reloaded.



## Example

First we create a new replication controller and service
```
$ kubectl create -f examples/echoheaders.yaml
replicationcontroller "echoheaders" created
You have exposed your service on an external port on all nodes in your
cluster.  If you want to expose this service to the external internet, you may
need to set up firewall rules for the service port(s) (tcp:30302) to serve traffic.

See http://releases.k8s.io/HEAD/docs/user-guide/services-firewalls.md for more details.
service "echoheaders" created
```

Next add the required annotation to expose the service using a local IP

```
$ kubectl annotate svc echoheaders "k8s.io/public-vip=10.4.0.50"
service "echoheaders" annotated

$ kubectl get svc echoheaders -o yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    k8s.io/public-vip: 10.4.0.50
  creationTimestamp: 2015-12-30T19:15:32Z
  labels:
    app: echoheaders
  name: echoheaders
  namespace: default
  resourceVersion: "50811"
  selfLink: /api/v1/namespaces/default/services/echoheaders
  uid: b26d82b8-af29-11e5-a436-b8aeed77f611
spec:
  clusterIP: 10.3.0.118
  ports:
  - name: http
    nodePort: 30302
    port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    app: echoheaders
  sessionAffinity: None
  type: NodePort
status:
  loadBalancer: {}
```

Now the creation of the daemonset
```
$ kubectl create -f vip-daemonset.yaml
daemonset "kube-keepalived-vip" created
$ kubectl get daemonset
NAME                  CONTAINER(S)          IMAGE(S)                         SELECTOR                        NODE-SELECTOR
kube-keepalived-vip   kube-keepalived-vip   gcr.io/google_containers/kube-keepalived-vip:0.1   name in (kube-keepalived-vip)   type=worker
```

**Note: the daemonset yaml file contains a node selector. This is not required, is just an example to show how is possible to limit the nodes where keepalived can run**

To verify if everything is working we should check if a `kube-keepalived-vip` pod is in each node of the cluster
```
$ kubectl get nodes
NAME       LABELS                                        STATUS    AGE
10.4.0.3   kubernetes.io/hostname=10.4.0.3,type=worker   Ready     1d
10.4.0.4   kubernetes.io/hostname=10.4.0.4,type=worker   Ready     1d
10.4.0.5   kubernetes.io/hostname=10.4.0.5,type=worker   Ready     1d
```

```
$ kubectl get pods
NAME                        READY     STATUS    RESTARTS   AGE
echoheaders-co4g4           1/1       Running   0          5m
kube-keepalived-vip-a90bt   1/1       Running   0          53s
kube-keepalived-vip-g3nku   1/1       Running   0          52s
kube-keepalived-vip-gd18l   1/1       Running   0          54s
```

```
$ kubectl logs kube-keepalived-vip-a90bt
I1230 19:52:24.790134       1 main.go:74] starting LVS configuration
I1230 19:52:24.873084       1 controller.go:166] Sync triggered by service default/echoheaders
I1230 19:52:24.973776       1 controller.go:168] Requeuing default/echoheaders because of error: deferring sync till endpoints controller has synced
I1230 19:52:24.973861       1 controller.go:166] Sync triggered by service default/kubernetes
I1230 19:52:24.973974       1 controller.go:134] Found service: echoheaders
I1230 19:52:24.975492       1 keepalived.go:110] {"authPass":"166ee42274df2b6b2fd2f23b1aed43df552d88ad","iface":"enp3s0","myIP":"10.4.0.4","netmask":24,"nodes":["10.4.0.3","10.4.0.5"],"priority":101,"svcs":[{"Name":"default/echoheaders","Ip":"10.4.0.50","Port":80,"Protocol":"TCP","Backends":[{"Ip":"10.2.48.2","Port":8080}]}]}
I1230 19:52:24.976034       1 keepalived.go:135] reloading keepalived
E1230 19:52:24.978096       1 controller.go:155] error reloading keepalived: exit status 1
I1230 19:52:24.978149       1 controller.go:166] Sync triggered by service default/echoheaders
I1230 19:52:24.978220       1 controller.go:134] Found service: echoheaders
I1230 19:52:24.978981       1 keepalived.go:110] {"authPass":"166ee42274df2b6b2fd2f23b1aed43df552d88ad","iface":"enp3s0","myIP":"10.4.0.4","netmask":24,"nodes":["10.4.0.3","10.4.0.5"],"priority":101,"svcs":[{"Name":"default/echoheaders","Ip":"10.4.0.50","Port":80,"Protocol":"TCP","Backends":[{"Ip":"10.2.48.2","Port":8080}]}]}
I1230 19:52:24.979647       1 keepalived.go:135] reloading keepalived
E1230 19:52:24.981501       1 controller.go:155] error reloading keepalived: exit status 1
I1230 19:52:29.867940       1 main.go:81] starting keepalived to announce VIPs
Starting Healthcheck child process, pid=18
Starting VRRP child process, pid=19
Initializing ipvs 2.6
Netlink reflector reports IP 10.4.0.4 added
Netlink reflector reports IP 10.4.0.50 added
Netlink reflector reports IP 10.2.48.0 added
Netlink reflector reports IP 10.2.48.1 added
Netlink reflector reports IP fe80::baae:edff:fe77:ef88 added
Netlink reflector reports IP fe80::f455:a1ff:fecc:3fc2 added
Netlink reflector reports IP fe80::42:e9ff:fec1:a2ad added
Netlink reflector reports IP fe80::609e:77ff:feae:ba7d added
Registering Kernel netlink reflector
Netlink reflector reports IP 10.4.0.4 added
Registering Kernel netlink command channel
Netlink reflector reports IP 10.4.0.50 added
Registering gratuitous ARP shared channel
Netlink reflector reports IP 10.2.48.0 added
Netlink reflector reports IP 10.2.48.1 added
Netlink reflector reports IP fe80::baae:edff:fe77:ef88 added
Netlink reflector reports IP fe80::f455:a1ff:fecc:3fc2 added
Netlink reflector reports IP fe80::42:e9ff:fec1:a2ad added
Netlink reflector reports IP fe80::609e:77ff:feae:ba7d added
Opening file '/etc/keepalived/keepalived.conf'.
Registering Kernel netlink reflector
Registering Kernel netlink command channel
Truncating auth_pass to 8 characters
Opening file '/etc/keepalived/keepalived.conf'.
Configuration is using : 68505 Bytes
Using LinkWatch kernel netlink reflector...
VRRP_Instance(vips) Entering BACKUP STATE
VRRP sockpool: [ifindex(2), proto(51), unicast(0), fd(10,11)]
Configuration is using : 14226 Bytes
Using LinkWatch kernel netlink reflector...
Activating healthchecker for service [10.2.48.2]:8080
TCP connection to [10.2.48.2]:8080 success.
Adding service [10.2.48.2]:8080 to VS [10.4.0.50]:80
Gained quorum 1+0=1 <= 1 for VS [10.4.0.50]:80
VRRP_Instance(vips) Transition to MASTER STATE
VRRP_Group(VG_1) Syncing instances to MASTER state
VRRP_Instance(vips) Entering MASTER STATE
VRRP_Instance(vips) setting protocol VIPs.
VRRP_Instance(vips) Sending gratuitous ARPs on enp3s0 for 10.4.0.50
```

```
$ kubectl exec kube-keepalived-vip-a90bt cat /etc/keepalived/keepalived.conf

vrrp_sync_group VG_1
  group {
    vips
  }
}

vrrp_instance vips {
  state BACKUP
  interface enp3s0
  virtual_router_id 50
  priority 100
  nopreempt
  advert_int 1

  track_interface {
    enp3s0
  }

  virtual_ipaddress {
    10.4.0.50
  }

  authentication {
    auth_type AH
    auth_pass 166ee42274df2b6b2fd2f23b1aed43df552d88ad
  }
}


virtual_server 10.4.0.50 80 {
  delay_loop 5
  lvs_sched wlc
  lvs_method NAT
  persistence_timeout 1800
  protocol TCP
  #TCP
  alpha


  real_server 10.2.48.2 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

}
```


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

Scaling the replication controller should update and reload keepalived

```
$ kubectl scale --replicas=5 replicationcontroller echoheaders
replicationcontroller "echoheaders" scaled
```


````
$ kubectl exec kube-keepalived-vip-a90bt cat /etc/keepalived/keepalived.conf

vrrp_sync_group VG_1
  group {
    vips
  }
}

vrrp_instance vips {
  state BACKUP
  interface enp3s0
  virtual_router_id 50
  priority 101
  nopreempt
  advert_int 1

  track_interface {
    enp3s0
  }

  virtual_ipaddress {
    10.4.0.50
  }

  authentication {
    auth_type AH
    auth_pass 166ee42274df2b6b2fd2f23b1aed43df552d88ad
  }
}


virtual_server 10.4.0.50 80 {
  delay_loop 5
  lvs_sched wlc
  lvs_method NAT
  persistence_timeout 1800
  protocol TCP
  #TCP
  alpha


  real_server 10.2.16.2 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.16.3 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.224.2 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.224.3 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

  real_server 10.2.48.2 8080 {
    weight 1
    TCP_CHECK {
      connect_port 8080
      connect_timeout 3
    }
  }

}
```


## TODO
- option to choose wich IP should be used (kubernetes VIP or pod IP)
- custom weight?
- quorum rules

## Related projects

- https://github.com/kobolog/gorb
- https://github.com/qmsk/clusterf
- https://github.com/osrg/gobgp
