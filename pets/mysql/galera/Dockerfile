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

# TODO: Move off ubuntu, it's the easiest to get off the ground with for mysql
# installation.
FROM ubuntu:14.04
MAINTAINER Prashanth.B <beeps@google.com>
ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update
RUN apt-get install -y software-properties-common
RUN apt-key adv --recv-keys --keyserver hkp://keyserver.ubuntu.com:80 BC19DDBA
RUN add-apt-repository 'deb http://releases.galeracluster.com/ubuntu trusty main'
RUN apt-get update
RUN apt-get install -y galera-3 bash galera-arbitrator-3 mysql-wsrep-5.6 rsync lsof wget
COPY my.cnf /etc/mysql/my.cnf
# TODO: Split out wsrep from my.cnf into a cluster.cnf

RUN wget -qO /peer-finder https://storage.googleapis.com/kubernetes-release/pets/peer-finder
RUN chmod -c 755 /peer-finder
ADD on-change.sh /
ADD on-start.sh /

EXPOSE 9376 3306 4444 4567 4568
ENTRYPOINT ["/peer-finder"]
