# DNS Performance Testing

## Test Methodology

We use the DNS Perf suite running simultaneously in N pods to stress the Kube DNS service.

### DNS Perf sources

You can browse the sources here.
```
ftp://ftp.nominum.com/pub/nominum/dnsperf/2.0.0.0/dnsperf-src-2.0.0.0-1.tar.gz
```

The DNS Perf suite needs a query file as input, we use their example published query file.

### Input Query File
You can see the input query file here.
```
ftp://ftp.nominum.com/pub/nominum/dnsperf/data/queryfile-example-current.gz
```

## Running the tests

Run 'make runtests' to launch the Kubernetes job for dnsperf - the Job runs the specified number of pods/completions that actually run the tests.

Monitor the pod logs to wait till the dns performance tests finish and print out the QPS and other summaries.

## Scaling

Change the completions and parallelism parameters in the dnsperf-job.yaml file to increase the number of test pods.

## Automating collection of results

TODO - Currently, it is up to the user to print the dnsperf container logs to gather the QPS and other stats printed at the end of the dnsperf runs. It should be possible to write a bash script to collect this automatically, trimmed and neatly formatted.

## Tuning DNS Perf suite parameters

The dnsperf suite has a large number of parameters that can be tuned - the current choice is to run 16 parallel clients and a 100 second maximum runtime.
