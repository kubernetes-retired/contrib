# HAProxy Ingress Controller

This is an haproxy Ingress controller.
The implementation doesn't extends Ingress object using annotations nor config maps for now.
This means that the HAProxy Ingress controller supports a subset of the scenarios covered by Nginx controller.

Although future work can bridge that gap keep in mind that
* HAProxy, Nginx and any other ingress controllers could share a common backend that communicates with kubernetes
* Ingress object will surely evolve to support other scenarios (L4, Acme)

## Building

`Makefile` at the root is self descriptive. Usually you'll want to `make linux` or `make darwin`.

## Executing

Command line parameters can be consulted at `cmd/main.go`

There must be a working kubernetes client configuration.
That should be the case when executing inside a pod in a cluster. If executing out of cluster, use `kubectl cluster-info` to make sure your environment is configured.

Use `-h` flag to make get the full argument list

```
$ haddock -h
Usage of :
      --alsologtostderr value           log to standard error as well as files
      --api-port int                    Port to expose the Ingress Controller API (default 8207)
      --balancer-ip string              Public IP Address where the balancer is located
      --balancer-pod-name string        balancer pod name
      --balancer-pod-namespace string   namespace where the balancer is running (default "default")
      --balancer-script string          Load balancer shell script that accepts commands as parameters
      --certs-dir string                Directory to store TLS certificates
      --config-file string              Load Balancer configuration file location
      --config-map string               Configuration for global balancer items (default "loadbalancer-conf")
      --ingress-class-filter value      Group of filter values for kubernetes.io/ingress.class ingress annotations accepted by this balancer. (default [])
      --ingress-class-needed            If set to true, will only process annotations indicated by 'ingress-class-filter'
      --listen string                   IP to expose Haddok API (default "0.0.0.0")
      --load-balancer string            Load Balancer to configure. For now only 'haproxy' is supported (default "haproxy")
      --log_backtrace_at value          when logging hits line file:N, emit a stack trace (default :0)
      --log_dir value                   If non-empty, write log files in this directory
      --logtostderr value               log to standard error instead of files
      --out-of-cluster                  If true the cluster balancer is not within the cluster
      --stderrthreshold value           logs at or above this threshold go to stderr (default 2)
      --sync-period duration            Relist Ingress frequency (default 30s)
  -v, --v value                         log level for V logs
      --vmodule value                   comma-separated list of pattern=N settings for file-filtered logging
      --watch-namespace string          namespace to watch
```

### In cluster example

Taken from the `docker-entrypoint.sh` file, the most common arguments when executing from within a pod are:

```
$ haddock \
  --config-file $BALANCER_CFG \
  --balancer-script $BALANCER_DIR/scripts/haproxy.sh \
  --certs-dir $BALANCER_DIR/certs \
  --api-port $BALANCER_API_PORT \
  --ingress-class-filter haproxy  --ingress-class-filter haddock \
  --balancer-ip $BALANCER_IP
```

The example uses
- The file location where haproxy configuration should be written
- Balancer script that controls haproxy binary. Accepts start, stop, restart and reload.
- Directory location where balancer certificates should be written
- Port where the controller will expose a simple API. Kubernetes healtz checks should point to this API
- Ingress class filter, in case we have multiple ingress controllers and want to process annotated ingress only
- Balancer IP so that ingress object can be updated with the external IP where requests should be sent

### Out of cluster example

Ingress controller configures HAProxy to use pod endpoints as upstream servers, which requires that the balancer can route to pods.
If you are planing HAProxy out of cluster and you are using an overlay network, you will need to extend that overlay to the balancer host, or any other equivalent solution.

```
$ haddock --balancer-ip 10.0.1.1 \
    --out-of-cluster \
    --config-file ./haproxy.cfg \
    --certs-dir ./certs \
    --balancer-script ./scripts/haproxy.sh \
    --ingress-class-filter haproxy  --ingress-class-filter haddock \
    --api-port 8207
```

## Building a kubernetes image

This repo contains an [example](image/) of a Dockerfile that builds an image that contains the Ingress Controller, HAProxy, scripts and configuration files.
It expects an additional subfolder with the binary `image/bin/haddock`. You can generate the binay and image customizing `Makefile` image repo and executing `make image`

## Examples

To create all Ingress examples follow this steps:

1. create the [balancer](./examples/balancer/README.md)
2. create the [backends](./examples/backends/README.md)
3. create the [ingress](./examples/ingress/README.md)
4. create the [TLS](./examples/tls/README.md)

Each of those the documents above contains an explanation of the resources created at each step
