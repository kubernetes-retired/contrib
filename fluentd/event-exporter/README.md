# Event Exporter

This tool is used to export Kubernetes events. It effectively runs a watch on
the apiserver, detecting as granular as possible all changes to the event
objects. Event exporter currently exports only to Stackdriver.

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
-sink-type string
    Name of the sink used for events ingestion (default "stackdriver")
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
        image: gcr.io/google.com/vmik-k8s-testing-0/event-exporter:v0.1.0
        command:
        - '/event-exporter'
```

Note, that this pod's service account should be authorized to get events, you
might need to set up ClusterRoleBinding in order to make it possible.
