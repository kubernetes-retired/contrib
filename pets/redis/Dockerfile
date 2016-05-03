# Copyright 2016 The Kubernetes Authors All rights reserved.
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

# TODO: Move off ubuntu, though even the official redis seems to be based on
# ubuntu.
FROM ubuntu:14.04
MAINTAINER Prashanth.B <beeps@google.com>

RUN apt-get update && apt-get install -y redis-server wget

RUN wget -qO /peer-finder https://storage.googleapis.com/kubernetes-release/pets/peer-finder
RUN chmod -c 755 /peer-finder
ADD on-start.sh /

EXPOSE 9376 6376
ENTRYPOINT ["/peer-finder"]
