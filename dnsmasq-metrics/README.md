# dnsmasq-metrics

`dnsmasq-metrics` is a daemon that exports metrics tracked by dnsmasq as
prometheus metrics. It is meant to be run in the same pod as a `dnsmasq`
instance.

## Building

* `make all` to build the executable.
* `make all-container` to build the Docker image.
* `make all-push` to push the image to the public repository.
* `make test` to run unit tests.
* `bash test/e2e/e2e.sh` will run an end-to-end test involving `dnsmasq` and
  `dnsmasq-metrics`. The test script should exit with no error.

## Running

`dnsmasq-metrics` is configured through command line flags, defaults of which
can be found by executing it with `--help`. Important flags to configure:

| Flag | Description |
| ---- | ---- |
| --dnsmasq-\{addr,port\} | endpoint of dnsmasq DNS service |
| --prometheus-\{addr,port\} | endpoint that dnsmasq-metrics will bind to export metrics |
