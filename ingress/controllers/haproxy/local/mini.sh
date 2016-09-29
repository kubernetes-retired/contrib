#!/bin/bash

cd "$(dirname "$0")"

../release/darwin/amd64/haddock --balancer-ip 10.1.1.1 \
    --out-of-cluster \
    --config-file ./haproxy.cfg \
    --certs-dir ./certs \
    --balancer-script ./scripts/haproxy.sh \
    --ingress-class-filter haproxy  --ingress-class-filter haddock \
    --api-port 8207
