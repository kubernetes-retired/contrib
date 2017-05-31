# pod-autoscaler

This container image watches over the number of schedulable nodes in the cluster and resizes 
the number of replicas in its parent object. Works only for pods which are children of RCs or RSs objects.

Usage of pod_nanny:
    --configFile <params file>
```

## Example rc file

The following yaml is an example Replication Controller where the nannies in all pods watch and resizes the RC replicas

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: pod-autoscaler-nanny-params
  namespace: kube-system
data:
  params.conf: |-
    nodes.gte.1=1
    nodes.gte.2=2
    nodes.gte.5=3
    nodes.gte.25=4
    nodes.gte.50=5
    nodes.gte.100=6
    nodes.gte.200=7
    nodes.gte.500=10
    nodes.gte.750=15
    nodes.gte.1000=25
    nodes.gte.1250=35
    nodes.gte.1500=45
    nodes.gte.1750=55
    nodes.gte.2000=65
    nodes.gte.2500=100
---
apiVersion: v1
kind: ReplicationController
metadata:
  name: pod-autoscaler-nanny
  namespace: default
  labels:
    k8s-app: pod-autoscaler-nanny
    version: v1
spec:
  replicas: 1
  template:
    metadata:
      labels:
        k8s-app: pod-autoscaler-nanny
        version: v1
    spec:
      volumes:
        - name: config-volume
          configMap:
            name: pod-autoscaler-nanny-params
      containers:
        - image: gcr.io/google_containers/dns-rc-autoscaler:0.5
          imagePullPolicy: Always
          name: pod-autoscaler-nanny
          resources:
            limits:
              cpu: 40m
              memory: 200Mi
            requests:
              cpu: 10m
              memory: 30Mi
          volumeMounts:
            - name: config-volume
              mountPath: /etc/pod_nanny
          env:
            - name: MY_POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: MY_POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          command:
            - /dns_pod_nanny
            - --paramsFile
            - /etc/pod_nanny/params.conf
            - --verbose


```
