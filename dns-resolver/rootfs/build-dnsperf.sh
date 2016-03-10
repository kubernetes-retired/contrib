#!/bin/sh

# Copyright 2015 The Kubernetes Authors All rights reserved.
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

apt-get update && apt-get install \
  libbind-dev \
  libkrb5-dev \
  libssl-dev \
  libcap-dev \
  libxml2-dev \
  bind9utils \
  make

cd /tmp
# Installing dnsperf
curl -sSL ftp://ftp.nominum.com/pub/nominum/dnsperf/2.1.0.0/dnsperf-src-2.1.0.0-1.tar.gz -o dnsperf-src-2.1.0.0-1.tar.gz
tar xfvz dnsperf-src-2.1.0.0-1.tar.gz
cd dnsperf-src-2.1.0.0-1
./configure
make
make install

cd /
