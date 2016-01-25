#!/usr/bin/env bash

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

set -e

# https://github.com/kubernetes/kubernetes/issues/14394
echo "Using test from https://github.com/kubernetes/kubernetes/issues/14394"

PORT="${1}"
TYPE="${2}"

echo "Checking hosts..."

DATA=$(cat ./$TYPE-data.txt)
DNSRECORDS=$(echo "${DATA}" | sed -e 's/$/\. A/g')

echo "${DATA}" | while read line
do
  if [ ! -z $line ]; then
    echo "host $line"
    if ! nslookup -port=$PORT $line;then
      echo "Invalid server: $line. Aborting"
      exit 1
    fi
  fi
done

echo "${DNSRECORDS}" > dns.txt

echo "Starting dns benchmark..."
dnsperf -s 127.0.0.1 -d ./dns.txt -l 300 -c 1000 -p $PORT
rm dns.txt
