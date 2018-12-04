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

FROM k8s.gcr.io/ubuntu-slim:0.5

RUN apt-get update && apt-get install -y --no-install-recommends bash

COPY build.sh /build.sh

ENV VERSION 2.0.10
ENV SHA256 8cfd31307e69fb6fc3417da4c67de8040b97c3e5fc67ebd7ffbe6c53aef22c8b

RUN /build.sh
