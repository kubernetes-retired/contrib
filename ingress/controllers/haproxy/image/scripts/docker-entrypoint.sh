#!/bin/sh
set -e


if [ "$1" = '' ]; then
  $BALANCER_DIR/bin/haddock \
    --config-file $BALANCER_CFG \
    --balancer-script $BALANCER_DIR/scripts/haproxy.sh \
    --certs-dir $BALANCER_DIR/certs \
    --api-port $BALANCER_API_PORT \
    --ingress-class-filter haproxy  --ingress-class-filter haddock \
    --balancer-pod-namespace $POD_NAMESPACE \
    --balancer-pod-name $POD_NAME \
    --balancer-ip $BALANCER_IP
else
  exec "$@"
fi
