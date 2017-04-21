**The Ingress controller examples have moved to the
[kubernetes/ingress](https://github.com/kubernetes/ingress) repository.**

In some cases could be required to run the Ingress controller in all the nodes in cluster.
Using [DaemonSet](https://github.com/kubernetes/kubernetes/blob/master/docs/design/daemon.md) it is possible to do this.
The file `as-daemonset.yaml` contains an example

```
kubectl create -f as-daemonset.yaml
```
