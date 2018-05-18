**The Ingress controller examples have moved to the
[kubernetes/ingress](https://github.com/kubernetes/ingress-nginx) repository.**

In some cases could be required to run the Ingress controller in all the nodes in cluster.
Using [DaemonSet](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/apps/daemon.md) it is possible to do this.
The file `as-daemonset.yaml` contains an example

```
kubectl create -f as-daemonset.yaml
```
