# addon-resizer

This container image watches over another container in a deployment, and
vertically scales the dependent container up and down. Currently the only
option is to scale it linearly based on the number of nodes, and it only works
for a singleton.

## Nanny program and arguments

The nanny scales resources linearly with the number of nodes in the cluster. The base and marginal resource requirements are given as command line arguments, but you cannot give a marginal requirement without a base requirement.

The cluster size is periodically checked, and used to calculate the expected resources. If the expected and actual resources differ by more than the threshold (given as a +/- percent), then the deployment is updated (updating a deployment stops the old pod, and starts a new pod).

```
Usage of pod_nanny:
      --container="pod-nanny": The name of the container to watch. This defaults to the nanny itself.
      --cpu="MISSING": The base CPU resource requirement.
      --deployment="": The name of the deployment being monitored. This is required.
      --extra-cpu="0": The amount of CPU to add per node.
      --extra-memory="0Mi": The amount of memory to add per node.
      --extra-storage="0Gi": The amount of storage to add per node.
      --log-flush-frequency=5s: Maximum number of seconds between log flushes
      --memory="MISSING": The base memory resource requirement.
      --namespace=$MY_POD_NAMESPACE: The namespace of the ward. This defaults to the nanny's own pod.
      --pod=$MY_POD_NAME: The name of the pod to watch. This defaults to the nanny's own pod.
      --poll-period=10000: The time, in milliseconds, to poll the dependent container.
      --storage="MISSING": The base storage resource requirement.
      --threshold=0: A number between 0-100. The dependent's resources are rewritten when they deviate from expected by more than threshold.
```

## Example deployment file

The following yaml is an example deployment where the nanny watches and resizes itself.

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nanny-v1
  namespace: default
  labels:
    k8s-app: nanny
    version: v1
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: nanny
      version: v1
  template:
    metadata:
      labels:
        k8s-app: nanny
        version: v1
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
        - image: gcr.io/google_containers/addon-resizer:1.0
          imagePullPolicy: Always
          name: pod-nanny
          resources:
            limits:
              cpu: 300m
              memory: 200Mi
            requests:
              cpu: 300m
              memory: 200Mi
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
            - /pod_nanny
            - --cpu=300m
            - --extra-cpu=20m
            - --memory=200Mi
            - --extra-memory=10Mi
            - --threshold=5
            - --deployment=nanny-v1
```
