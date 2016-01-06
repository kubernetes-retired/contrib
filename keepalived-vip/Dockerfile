# Copyright 2015 The Kubernetes Authors. All rights reserved.
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

FROM alpine:3.2

COPY keepalived.apk /var/cache/apk/keepalived.apk

RUN apk add -U --repository http://dl-3.alpinelinux.org/alpine/edge/testing/ --allow-untrusted \
  bash curl ipvsadm iproute2 && \
  apk add --allow-untrusted /var/cache/apk/keepalived.apk && \
  rm -rf /var/cache/apk/* /etc/keepalived/keepalived.conf
  
COPY kube-keepalived-vip /kube-keepalived-vip

ENTRYPOINT ["/kube-keepalived-vip"]
