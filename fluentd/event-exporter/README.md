# Event Exporter

This tool is used to export Kubernetes events. It effectively runs a watch on
the apiserver, detecting as granular as possible all changes to the event
objects. Event exporter exports only to Stackdriver.

## Build

To build the binary, run

```shell
make build
```

To run unit tests, run

```shell
make test
```

To build the container, run

```shell
make container
```

## Run

Event exporter has following options:

```
-prometheus-endpoint string
    Endpoint on which to expose Prometheus http handler (default ":80")
-resync-period duration
    Reflector resync period (default 1m0s)
-sink-opts string
    Parameters for configuring sink
```

Set of flags for configuring sink is the following:

```
Usage of stackdriver:
  -flush-delay duration
      Delay after receiving the first event in batch before sending the request to Stackdriver, if batchdoesn't get sent before (default 5s)
  -max-buffer-size int
      Maximum number of events in the request to Stackdriver (default 100)
  -max-concurrency int
      Maximum number of concurrent requests to Stackdriver (default 10)
```

## Deploy

Example deployment:

```yaml
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: event-exporter-deployment
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: event-exporter
    spec:
      containers:
      - name: event-exporter
        image: gcr.io/google-containers/event-exporter:v0.1.4
        command:
        - '/event-exporter'
```

Note, that this pod's service account should be authorized to get events, you
might need to set up ClusterRoleBinding in order to make it possible. Complete
example with the service account and the cluster role binding you can find in
the `example` directory.
