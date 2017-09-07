#!/bin/sh

# Copyright 2015 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
