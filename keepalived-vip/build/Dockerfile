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

FROM gcr.io/google_containers/ubuntu-slim:0.3

RUN apt-get update && apt-get install -y --no-install-recommends bash

COPY build.sh /build.sh

ENV VERSION 1.2.22
ENV SHA256 aee637059410c1ef94e1c9d2e96274e375ed4d05665d2b28c13275e0fc951839

RUN /build.sh
