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

FROM gcr.io/google_containers/ubuntu-slim:0.6

RUN apt-get update && apt-get install -y --no-install-recommends bash

COPY build.sh /build.sh

ENV VERSION 1.3.2
ENV SHA256 1790d9bf4c848ba4c39ae3b28ce0ee763614bb5db2ce3e8856ea00cf6b105367

RUN /build.sh
