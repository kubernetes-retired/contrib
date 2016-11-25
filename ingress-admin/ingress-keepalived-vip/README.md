# ingress-keepalived-vip

## Overview

Ingress can be used to expose a service in the kubernetes cluster:

cluster admin deploys an ingress-controller Pod beforehand user creates Ingress
resource the ingress-controller Pod list&watch All Ingress Resources in the
cluster, when it sees a new Ingress resource:

* on cloud provider, it calls the cloud provider to sync the ingress L7
  loadbalancing rules
* on bare-metal, it syncs nginx (or haproxy, etc) config then reload

user out of cluster can then access service in the cluster:

* on bare-metal, accessing the node's ip on which ingress-controller Pod is
  running, ingress-controller Pod will forward request into cluster based on
  rules defined in Ingress resource
* on cloud-provider, accessing the ip provided by cloud provider loadbalancer,
  cloud provider will forward request into cluster based on rules defined in
  Ingress Resource


This just works. What's the issue then?

The issue is: on bare-metal, it does not provide High Availability because
client needs to know the IP addresss of the node where ingress-controller
Pod is running. In case of a failure, the ingress-controller Pod will
be moved to a different node.

## ingress-keepalived-vip sidecar container


##### How it works
* cluster admin choose a group of nodes which could be accessed out of cluster
  and are in the same L2 broacast domain to run Ingress Pod
* deploy Ingress Pod using ReplicaSet(at least 2 replicas for HA)
* using AntiAffinity feature so that Ingress Pod created by the same Ingress
  ReplicaSet could be scheduled to different node
* Create a Service, which is backended by the Ingress ReplicaSet
* Put the VIP as an annotation of the Service
* Ingress Pods use host network
* Ingress Pods created by the same Ingress ReplicaSet will run keepalived
  only one Ingress Pod will get the VIP
* users out of cluster access incluster service by the Ingress VIP

##### Examples
```
apiVersion: v1
kind: Service
metadata:
  name: ingress
  namespace: kube-system
  annotations:
    "ingress.alpha.k8s.io/ingress-vip": "192.168.10.1"
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
```


```
apiVersion: v1
kind: ReplicationController
metadata:
  namespace: "kube-system"
  name: "ingress-test1"
  labels:
    run: "ingress"
spec:
  replicas: 2
  selector:
    run: "ingress"
  template:
   metadata:
    labels:
      run: "ingress"
   spec:
    hostNetwork: true
    containers:
      - image: "keepalived-vip:v0.0.1"
        imagePullPolicy: "Always"
        name: "keepalived"
        resources:
          limits:
            cpu: 50m
            memory: 100Mi
          requests:
            cpu: 10m
            memory: 10Mi
        securityContext:
          privileged: true
        env:
          - name: POD_NAME
            valueFrom:
              fieldRef:
                fieldPath: metadata.name
          - name: POD_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: SERVICE_NAME
            value: "ingress"
      - image: "ingress-controller:v0.0.1"
        imagePullPolicy: "Always"
        name: "ingress-controller"
        resources:
          limits:
            cpu: 200m
            memory: 200Mi
          requests:
            cpu: 200m
            memory: 200Mi

```
