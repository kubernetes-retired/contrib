# Copyright 2015 Google Inc. All rights reserved.
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

FROM google/debian:wheezy
MAINTAINER Brendan Burns <bburns@google.com>
RUN apt-get update
RUN apt-get install -y -qq ca-certificates
ADD mungegithub /mungegithub
ADD blunderbuss.yml /blunderbuss.yml
ADD path-label.txt /path-label.txt
ADD run.sh /run.sh
RUN chmod a+x /run.sh
