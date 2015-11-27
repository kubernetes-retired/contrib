# GLBC

GLBC is a GCE L7 load balancer controller that manages external loadbalancers configured through the Kubernetes Ingress API.

## Disclaimer
- This is a **work in progress**.
- It relies on an experimental Kubernetes resource.
- The loadbalancer controller pod is not aware of your GCE quota.

## Overview

__A reminder on GCE L7__: Google Compute Engine does not have a single resource that represents a L7 loadbalancer. When a user request comes in, it is first handled by the global forwarding rule, which sends the traffic to an HTTP proxy service that sends the traffic to a URL map that parses the URL to see which backend service will handle the request. Each backend service is assigned a set of virtual machine instances grouped into instance groups.

__A reminder on Services__: A Kubernetes Service defines a set of pods and a means by which to access them, such as single stable IP address and corresponding DNS name. This IP defaults to a cluster VIP in a private address range. You can direct ingress traffic to a particular Service by setting its `Type` to NodePort or LoadBalancer. NodePort opens up a port on *every* node in your cluster and proxies traffic to the endpoints of your service, while LoadBalancer allocates an L4 cloud loadbalancer.

### L7 Load balancing on Kubernetes

To achive L7 loadbalancing through Kubernetes, we employ a resource called `Ingress`. The Ingress is consumed by this loadbalancer controller, which creates the following GCE resource graph:

[Global Forwarding Rule](https://cloud.google.com/compute/docs/load-balancing/http/global-forwarding-rules) -> [TargetHttpProxy](https://cloud.google.com/compute/docs/load-balancing/http/target-proxies) -> [Url Map](https://cloud.google.com/compute/docs/load-balancing/http/url-map) -> [Backend Service](https://cloud.google.com/compute/docs/load-balancing/http/backend-service) -> [Instance Group](https://cloud.google.com/compute/docs/instance-groups/)

The controller (glbc) manages the lifecycle of each component in the graph. It uses the Kubernetes resources as a spec for the desired state, and the GCE cloud resources as the observed state, and drives the observed to the desired. If an edge is disconnected, it fixes it. Each Ingress translates to a new GCE L7, and the rules on the Ingress become paths in the GCE Url Map. This allows you to route traffic to various backend Kubernetes Services through a single public IP, which is in contrast to `Type=LoadBalancer`, which allocates a public IP *per* Kubernetes Service. For this to work, the Kubernetes Service *must* have Type=NodePort.

### The Ingress

An Ingress in Kubernetes is a REST object, similar to a Service. A minimal Ingress might look like:

```yaml
01. apiVersion: extensions/v1beta1
02. kind: Ingress
03. metadata:
04.  name: hostlessendpoint
05. spec:
06.  rules:
07.  - http:
08.      paths:
09.      - path: /hostless
10.        backend:
11.          serviceName: test
12.          servicePort: 80
```

POSTing this to the Kubernetes API server would result in glbc creating a GCE L7 that routes all traffic sent to `http://ip-of-loadbalancer/hostless` to :80 of the service named `test`. If the service doesn't exist yet, or doesn't have a nodePort, glbc will allocate an IP and wait till it does. Once the Service shows up, it will create the required path rules to route traffic to it.

__Lines 1-4__: Resource metadata used to tag GCE resources. For example, if you go to the console you would see a url map called: k8-fw-default-hostlessendpoint, where default is the namespace and hostlessendpoint is the name of the resource. The Kubernetes API server ensures that namespace/name is unique so there will never be any collisions.

__Lines 5-7__: Ingress Spec has all the information needed to configure a GCE L7. Most importantly, it contains a list of `rules`. A rule can take many forms, but the only rule relevant to glbc is the `http` rule.

__Lines 8-9__: Each http rule contains the following information: A host (eg: foo.bar.com, defaults to `*` in this example), a list of paths (eg: `/hostless`) each of which has an associated backend (`test:80`). Both the `host` and `path` must match the content of an incoming request before the L7 directs traffic to the `backend`.

__Lines 10-12__: A `backend` is a service:port combination. It selects a group of pods capable of servicing traffic sent to the path specified in the parent rule.

__Global Prameters__: For the sake of simplicity the example Ingress has no global parameters. However, one can specify a default backend (see examples below) in the absence of which requests that don't match a path in the spec are sent to the default backend of glbc. Though glbc doesn't support HTTPS yet, security configs would also be global.


## Load Balancer Management

You can manage a GCE L7 by creating/updating/deleting the associated Kubernetes Ingress.

### Creation

Before you can start creating Ingress you need to start up glbc. We can use the rc.yaml in this directory:
```shell
$ kubectl create -f rc.yaml
replicationcontroller "glbc" created
$ kubectl get pods
NAME                READY     STATUS    RESTARTS   AGE
glbc-6m6b6          2/2       Running   0          21s

```

A couple of things to note about this controller:
* It needs a service with a node port to use as the default backend. This is the backend that's used when an Ingress does not specify the default.
* It has an intentionally long terminationGracePeriod, this is only required with the --delete-all-on-quit flag (see [Deletion](#deletion))
* Don't start 2 instances of the controller in a single cluster, they will fight each other.

The loadbalancer controller will watch for Services, Nodes and Ingress. Nodes already exist (the nodes in your cluster). We need to create the other 2. You can do so using the ingress-app.yaml in this directory.

A couple of things to note about the Ingress:
* It creates a Replication Controller for a simple echoserver application, with 1 replica.
* It creates 3 services for the same application pod: echoheaders[x, y, default]
* It creates an Ingress with 2 hostnames and 3 endpoints (foo.bar.com{/foo} and bar.baz.com{/foo, /bar}) that access the given service

```shell
$ kubectl create -f ingress-app.yaml
$ kubectl get svc
NAME                 CLUSTER_IP     EXTERNAL_IP   PORT(S)   SELECTOR          AGE
echoheadersdefault   10.0.43.119    nodes         80/TCP    app=echoheaders   16m
echoheadersx         10.0.126.10    nodes         80/TCP    app=echoheaders   16m
echoheadersy         10.0.134.238   nodes         80/TCP    app=echoheaders   16m
Kubernetes           10.0.0.1       <none>        443/TCP   <none>            21h

$ kubectl get ing
NAME      RULE          BACKEND                 ADDRESS
echomap   -             echoheadersdefault:80
          foo.bar.com
          /foo          echoheadersx:80
          bar.baz.com
          /bar          echoheadersy:80
          /foo          echoheadersx:80
```

You can tail the logs of the controller to observe its progress:
```
$ kubectl logs --follow glbc-6m6b6 l7-lb-controller
I1005 22:11:26.731845       1 instances.go:48] Creating instance group k8-ig-foo
I1005 22:11:34.360689       1 controller.go:152] Created new loadbalancer controller
I1005 22:11:34.360737       1 controller.go:172] Starting loadbalancer controller
I1005 22:11:34.380757       1 controller.go:206] Syncing default/echomap
I1005 22:11:34.380763       1 loadbalancer.go:134] Syncing loadbalancers [default/echomap]
I1005 22:11:34.380810       1 loadbalancer.go:100] Creating l7 default-echomap
I1005 22:11:34.385161       1 utils.go:83] Syncing e2e-test-beeps-minion-ugv1
...
```

When it's done, it will update the status of the Ingress with the ip of the L7 it created:
```shell
$ kubectl get ing
NAME      RULE          BACKEND                 ADDRESS
echomap   -             echoheadersdefault:80   107.178.254.239
          foo.bar.com
          /foo          echoheadersx:80
          bar.baz.com
          /bar          echoheadersy:80
          /foo          echoheadersx:80
```

Go to your GCE console and confirm that the following resources have been created through the HTTPLoadbalancing panel:
* A Global Forwarding Rule
* An UrlMap
* A TargetHTTPProxy
* BackendServices (one for each Kubernetes nodePort service)
* An Instance Group (with ports corresponding to the BackendServices)

The HTTPLoadBalancing panel will also show you if your backends have responded to the health checks, wait till they do. This can take a few minutes. If you see `Health status will display here once configuration is complete.` the L7 is still bootstrapping. Wait till you have `Healthy instances: X`. Even though the GCE L7 is driven by our controller, which notices the Kubernetes healtchecks of a pod, we still need to wait on the first GCE L7 health check to complete. Once your backends are up and healthy:

```shell
$ curl --resolve foo.bar.com:80:107.178.245.239 http://foo.bar.com/foo
CLIENT VALUES:
client_address=('10.240.29.196', 56401) (10.240.29.196)
command=GET
path=/echoheadersx
real path=/echoheadersx
query=
request_version=HTTP/1.1

SERVER VALUES:
server_version=BaseHTTP/0.6
sys_version=Python/3.4.3
protocol_version=HTTP/1.0

HEADERS RECEIVED:
Accept=*/*
Connection=Keep-Alive
Host=107.178.254.239
User-Agent=curl/7.35.0
Via=1.1 google
X-Forwarded-For=216.239.45.73, 107.178.254.239
X-Forwarded-Proto=http
```

You can also edit `/etc/hosts` instead of using `--resolve`.

#### Updates

Say you don't want a default backend and you'd like to allow all traffic hitting your loadbalancer at /foo to reach your echoheaders backend service, not just the traffic for foo.bar.com. You can modify the Ingress Spec:

```yaml
spec:
  rules:
  - http:
      paths:
      - path: /foo
..
```

and replace the existing Ingress (ignore errors about replacing the Service, we're using the same .yaml file but we only care about the Ingress):

```
$ kubectl replace -f ingress-app.yaml
ingress "echomap" replaced

$ curl http://107.178.254.239/foo
CLIENT VALUES:
client_address=('10.240.143.179', 59546) (10.240.143.179)
command=GET
path=/foo
real path=/foo
...

$ curl http://107.178.254.239/
<pre>
INTRODUCTION
============
This is an nginx webserver for simple loadbalancer testing. It works well
for me but it might not have some of the features you want. If you would
...
```

A couple of things to note about this particular update:
* An Ingress without a default backend inherits the backend of the Ingress controller.
* A IngressRule without a host gets the wildcard. This is controller specific, some loadbalancer controllers do not respect anything but a DNS subdomain as the host. You *cannot* set the host to a regex.
* You never want to delete then re-create an Ingress, as it will result in the controller tearing down and recreating the loadbalancer.

__Unexpected updates__: Since glbc constantly runs a control loop it won't allow you to break links that black hole traffic. An easy link to break is the url map itself, but you can also disconnect a target proxy from the urlmap, or remove an instance from the instance group (note this is different from *deleting* the instance, the loadbalancer controller will not recreate it if you do so). Modify one of the url links in the map to point to another backend through the GCE Control Panel UI, and wait till the controller sync (this happens as frequently as you tell it to, via the --resync-period flag). The same goes for the Kubernetes side of things, the API server will validate against obviously bad updates, but if you relink an Ingress so it points to the wrong backends the controller will blindly follow.

### Paths

Till now, our examples were simplified in that they hit an endpoint with a catch-all path regex. Most real world backends have subresources. Let's create service to test how the loadbalancer handles paths:
```yaml
apiVersion: v1
kind: ReplicationController
metadata:
  name: nginxtest
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: nginxtest
    spec:
      containers:
      - name: nginxtest
        image: bprashanth/nginxtest:1.0
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginxtest
  labels:
    app: nginxtest
spec:
  type: NodePort
  ports:
  - port: 80
    targetPort: 80
    protocol: TCP
    name: http
  selector:
    app: nginxtest
```

Running kubectl create against this manifest will given you a service with multiple endpoints:
```shell
$ kubectl get svc nginxtest -o yaml | grep -i nodeport:
    nodePort: 30404
$ curl nodeip:30404/
ENDPOINTS
=========
 <a href="hostname">hostname</a>: An endpoint to query the hostname.
 <a href="stress">stress</a>: An endpoint to stress the host.
 <a href="fs/index.html">fs</a>: A file system for static content.

```
You can put the nodeip:port into your browser and play around with the endpoints so you're familiar with what to expect. We will test the `/hostname` and `/fs/files/nginx.html` endpoints. Modify/create your Ingress:
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: nginxtest-ingress
spec:
  rules:
  - http:
      paths:
      - path: /hostname
        backend:
          serviceName: nginxtest
          servicePort: 80
```

And check the endpoint (you will have to wait till the update takes effect, this could be a few minutes):
```shell
$ kubectl replace -f ingress.yaml
$ curl loadbalancerip/hostname
nginx-tester-pod-name
```

Note what just happened, the endpoint exposes /hostname, and the loadbalancer forwarded the entire matching url to the endpoint. This means if you had '/foo' in the Ingress and tried accessing /hostname, your endpoint would've received /foo/hostname and not known how to route it. Now update the Ingress to access static content via the /fs endpoint:
```
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: nginxtest-ingress
spec:
  rules:
  - http:
      paths:
      - path: /fs/*
        backend:
          serviceName: nginxtest
          servicePort: 80
```

As before, wait a while for the update to take effect, and try accessing `loadbalancerip/fs/files/nginx.html`.

#### Deletion

Most production loadbalancers live as long as the nodes in the cluster and are torn down when the nodes are destroyed. That said, there are plenty of use cases for deleting an Ingress, deleting a loadbalancer controller, or just purging external loadbalancer resources alltogether. Deleting a loadbalancer controller pod will not affect the loadbalancers themselves, this way your backends won't suffer a loss of availability if the scheduler pre-empts your controller pod. Deleting a single loadbalancer is as easy as deleting an Ingress via kubectl:
```shell
$ kubectl delete ing echomap
$ kubectl logs --follow glbc-6m6b6 l7-lb-controller
I1007 00:25:45.099429       1 loadbalancer.go:144] Deleting lb default-echomap
I1007 00:25:45.099432       1 loadbalancer.go:437] Deleting global forwarding rule k8-fw-default-echomap
I1007 00:25:54.885823       1 loadbalancer.go:444] Deleting target proxy k8-tp-default-echomap
I1007 00:25:58.446941       1 loadbalancer.go:451] Deleting url map k8-um-default-echomap
I1007 00:26:02.043065       1 backends.go:176] Deleting backends []
I1007 00:26:02.043188       1 backends.go:134] Deleting backend k8-be-30301
I1007 00:26:05.591140       1 backends.go:134] Deleting backend k8-be-30284
I1007 00:26:09.159016       1 controller.go:232] Finished syncing default/echomap
```
Note that it takes ~30 seconds to purge cloud resources, the API calls to create and delete are a one time cost. GCE BackendServices are ref-counted and deleted by the controller as you delete Kubernetes Ingress'. This is not sufficient for cleanup, because you might have deleted the Ingress while glbc was down, in which case it would leak cloud resources. You can delete the glbc and purge cloud resources in 2 more ways:

__The dev/test way__: If you want to delete everything in the cloud when the loadbalancer controller pod dies, start it with the --delete-all-on-quit flag. When a pod is killed it's first sent a SIGTERM, followed by a grace period (set to 10minutes for loadbalancer controllers), followed by a SIGKILL. The controller pod uses this time to delete cloud resources. Be careful with --delete-all-on-quit, because if you're running a production glbc and the scheduler re-schedules your pod for some reason, it will result in a loss of availability. You can do this because your rc.yaml has:
```yaml
args:
# auto quit requires a high termination grace period.
- --delete-all-on-quit=true
```

So simply delete the replication controller:
```shell
$ kubectl get rc glbc
CONTROLLER   CONTAINER(S)           IMAGE(S)                                      SELECTOR                    REPLICAS   AGE
glbc         default-http-backend   gcr.io/google_containers/defaultbackend:1.0   k8s-app=glbc,version=v0.5   1          2m
             l7-lb-controller       gcr.io/google_containers/glbc:0.5

$ kubectl delete rc glbc
replicationcontroller "glbc" deleted

$ kubectl get pods
NAME                    READY     STATUS        RESTARTS   AGE
glbc-6m6b6              1/1       Terminating   0          13m
```

__The prod way__: If you didn't start the controller with `--delete-all-on-quit`, you can execute a GET on the `/delete-all-and-quit` endpoint. This endpoint is deliberately not exported.

```
$ kubectl exec -it glbc-6m6b6  -- curl http://localhost:8081/delete-all-and-quit
..Hangs till quit is done..

$ kubectl logs glbc-6m6b6  --follow
I1007 00:26:09.159016       1 controller.go:232] Finished syncing default/echomap
I1007 00:29:30.321419       1 controller.go:192] Shutting down controller queues.
I1007 00:29:30.321970       1 controller.go:199] Shutting down cluster manager.
I1007 00:29:30.321574       1 controller.go:178] Shutting down Loadbalancer Controller
I1007 00:29:30.322378       1 main.go:160] Handled quit, awaiting pod deletion.
I1007 00:29:30.321977       1 loadbalancer.go:154] Creating loadbalancers []
I1007 00:29:30.322617       1 loadbalancer.go:192] Loadbalancer pool shutdown.
I1007 00:29:30.322622       1 backends.go:176] Deleting backends []
I1007 00:30:00.322528       1 main.go:160] Handled quit, awaiting pod deletion.
I1007 00:30:30.322751       1 main.go:160] Handled quit, awaiting pod deletion
```

You just instructed the loadbalancer controller to quit, however if it had done so, the replication controller would've just created another pod, so it waits around till you delete the rc.

#### Health checks

Currently, all service backends must respond with a 200 on '/'. The content does not matter. If they fail to do so they will be deemed unhealthy by the GCE L7. This limitation is because there are 2 sets of health checks:
* From the kubernetes endpoints, taking the form of liveness/readiness probes
* From the GCE L7, which periodically pings '/'
We really want (1) to control the health of an instance but (2) is a GCE requirement. Ideally, we would point (2) at (1), but we still need (2) for pods that don't have a defined health check. This will probably get resolved when Ingress grows up.

## Troubleshooting:

This controller is complicated because it exposes a tangled set of external resources as a single logical abstraction. It's recommended that you are at least *aware* of how one creates a GCE L7 [without a kubernetes Ingress](https://cloud.google.com/container-engine/docs/tutorials/http-balancer). If weird things happen, here are some basic debugging guidelines:

* Check loadbalancer controller pod logs via kubectl
A typical sign of trouble is repeated retries in the logs:
```shell
I1006 18:58:53.451869       1 loadbalancer.go:268] Forwarding rule k8-fw-default-echomap already exists
I1006 18:58:53.451955       1 backends.go:162] Syncing backends [30301 30284 30301]
I1006 18:58:53.451998       1 backends.go:134] Deleting backend k8-be-30302
E1006 18:58:57.029253       1 utils.go:71] Requeuing default/echomap, err googleapi: Error 400: The backendService resource 'projects/Kubernetesdev/global/backendServices/k8-be-30302' is already being used by 'projects/Kubernetesdev/global/urlMaps/k8-um-default-echomap'
I1006 18:58:57.029336       1 utils.go:83] Syncing default/echomap
```

This could be a bug or quota limitation. In the case of the former, please head over to slack or github.

* If you see a GET hanging, followed by a 502 with the following response:

```
<html><head>
<meta http-equiv="content-type" content="text/html;charset=utf-8">
<title>502 Server Error</title>
</head>
<body text=#000000 bgcolor=#ffffff>
<h1>Error: Server Error</h1>
<h2>The server encountered a temporary error and could not complete your request.<p>Please try again in 30 seconds.</h2>
<h2></h2>
</body></html>
```
The loadbalancer is probably bootstrapping itself.

* If a GET responds with a 404 and the following response:
```
  <a href=//www.google.com/><span id=logo aria-label=Google></span></a>
  <p><b>404.</b> <ins>That’s an error.</ins>
  <p>The requested URL <code>/hostless</code> was not found on this server.  <ins>That’s all we know.</ins>
```
It means you have lost your IP somehow, or just typed in the wrong IP.

* If you see requests taking an abnormal amount of time, run the echoheaders pod and look for the client address
```shell
CLIENT VALUES:
client_address=('10.240.29.196', 56401) (10.240.29.196)
```

Then head over to the GCE node with internal ip 10.240.29.196 and check that the [Service is functioning](https://github.com/kubernetes/kubernetes/blob/release-1.0/docs/user-guide/debugging-services.md) as expected. Remember that the GCE L7 is routing you through the NodePort service, and try to trace back.

* Check if you can access the backend service directly via nodeip:nodeport
* Check the GCE console
* Make sure you only have a single loadbalancer controller running
* Make sure the initial GCE health checks have passed
* A crash loop looks like:
```shell
$ kubectl get pods
glbc-fjtlq             0/1       CrashLoopBackOff   17         1h
```
If you hit that it means the controller isn't even starting. Re-check your input flags, especially the required ones.

## GCELBC Implementation Details

For the curious, here is a high level overview of how the GCE LoadBalancer controller manages cloud resources.

The controller manages cloud resources through a notion of pools. Each pool is the representation of the last known state of a logical cloud resource. Pools are periodically synced with the desired state, as reflected by the Kubernetes api. When you create a new Ingress, the following happens:
* Create BackendServices for each Kubernetes backend in the Ingress, through the backend pool.
* Add nodePorts for each BackendService to an Instance Group with all the instances in your cluster, through the instance pool.
* Create a UrlMap, TargetHttpProxy, Global Forwarding Rule through the loadbalancer pool.
* Update the loadbalancer's urlmap according to the Ingress.

Periodically, each pool checks that it has a valid connection to the next hop in the above resource graph. So for example, the backend pool will check that each backend is connected to the instance group and that the node ports match, the instance group will check that all the Kubernetes nodes are a part of the instance group, and so on. Since Backends are a limited resource, they're shared (well, everything is limited by your quota, this applies doubly to backend services). This means you can setup N Ingress' exposing M services through different paths and the controller will only create M backends. When all the Ingress' are deleted, the backend pool GCs the backend.

## Wishlist:

* E2e, integration tests
* Better events
* Detect leaked resources even if the Ingress has been deleted when the controller isn't around
* Specify health checks (currently we just rely on kubernetes service/pod liveness probes and force pods to have a `/` endpoint that responds with 200 for GCE)
* Alleviate the NodePort requirement for Service Type=LoadBalancer.
* Async pool management of backends/L7s etc
* Retry back-off when GCE Quota is done
* GCE Quota integration
* HTTP support as the Ingress grows
* More aggressive resource sharing

[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/contrib/service-loadbalancer/gce/README.md?pixel)]()
