**The Ingress controller examples have moved to the
[kubernetes/ingress](https://github.com/kubernetes/ingress) repository.**

# Simple TLS example

Create secret
```console
$ make keys secret
$ kubectl create -f /tmp/tls.json
```

Make sure you have the l7 controller running:
```console
$ kubectl --namespace=kube-system get pod -l name=glbc
NAME
l7-lb-controller-v0.6.0-1770t ...
```
Also make sure you have a [firewall rule](https://kubernetes.io/docs/tasks/access-application-cluster/configure-cloud-provider-firewall/#google-compute-engine) for the node port of the Service.

Create Ingress
```console
$ kubectl create -f tls-app.yaml
```
