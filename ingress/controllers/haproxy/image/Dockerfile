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

FROM haproxy:1.6-alpine
MAINTAINER Matt Baldwin (baldwin@stackpointcloud.com)


ENV BALANCER_DIR /opt/stackpoint/balancer
ENV PATH $BALANCER_DIR/bin/:$PATH
ENV BALANCER_CFG $BALANCER_DIR/cfg/haproxy.cfg
ENV BALANCER_API_PORT 8207

RUN mkdir -p $BALANCER_DIR/cfg $BALANCER_DIR/bin $BALANCER_DIR/scripts $BALACNER_DIR/certs

WORKDIR $BALANCER_DIR/bin/

COPY ./files/haproxy.cfg $BALANCER_CFG
COPY ./bin/haddock $BALANCER_DIR/bin/
COPY ./scripts/haproxy.sh $BALANCER_DIR/scripts/
COPY ./scripts/docker-entrypoint.sh /


ENTRYPOINT ["/docker-entrypoint.sh"]
