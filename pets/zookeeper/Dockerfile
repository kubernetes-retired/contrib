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

FROM java:openjdk-8-jre
MAINTAINER Prashanth.B <beeps@google.com>

LABEL name="zookeeper" version="3.5.0-alpha"

RUN apt-get update && apt-get install -y wget bash dnsutils\
    && mkdir -p /opt \
    && wget -q -O - http://apache.mirrors.pair.com/zookeeper/zookeeper-3.5.0-alpha/zookeeper-3.5.0-alpha.tar.gz | tar -xzf - -C /opt \
    && mv /opt/zookeeper-3.5.0-alpha /opt/zookeeper \
    && cp /opt/zookeeper/conf/zoo_sample.cfg /opt/zookeeper/conf/zoo.cfg \
    && mkdir -p /tmp/zookeeper

RUN echo "standaloneEnabled=false" >> /opt/zookeeper/conf/zoo.cfg
RUN echo "dynamicConfigFile=/opt/zookeeper/conf/zoo.cfg.dynamic" >> /opt/zookeeper/conf/zoo.cfg

# TODO: This will not be necessary once we have init containers and notifications
RUN wget -qO /peer-finder https://storage.googleapis.com/kubernetes-release/pets/peer-finder
RUN chmod -c 755 /peer-finder
ADD on-change.sh /
ADD on-start.sh /

EXPOSE 9376 2181 2888 3888
WORKDIR /opt/zookeeper
ENTRYPOINT ["/peer-finder"]
