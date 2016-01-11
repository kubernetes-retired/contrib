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

RUN apk add -U alpine-sdk

RUN mkdir -p /var/cache/distfiles && \
  adduser -D packager && \
    addgroup packager abuild && \
    chgrp abuild /var/cache/distfiles && \
    chmod g+w /var/cache/distfiles && \
    echo "packager    ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

USER packager

WORKDIR /work

RUN abuild-keygen -a -i
ADD src /work
RUN sudo chown -R packager /work

RUN abuild -c -r -P /work