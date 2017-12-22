**The Ingress controller examples have moved to the
[kubernetes/ingress](https://github.com/kubernetes/ingress) repository.**

All the examples references the services `echoheaders-x` and `echoheaders-y`

```
kubectl run echoheaders --image=k8s.gcr.io/echoserver:1.4 --replicas=1 --port=8080
kubectl expose deployment echoheaders --port=80 --target-port=8080 --name=echoheaders-x
kubectl expose deployment echoheaders --port=80 --target-port=8080 --name=echoheaders-y
```
