#!/bin/bash

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

set -e

export SERVER=$1
export CLIENT_NUM=$2
export RUNTIME=$3
export QUERYFILE=$4

echo "Running DNS Perf Job - DNS Server is $SERVER, Clients is $CLIENT_NUM and Runtime is $RUNTIME, file is $QUERYFILE"

/usr/local/bin/dnsperf -s $SERVER -d $QUERYFILE -S 10 -t 15 -c $CLIENT_NUM -l $RUNTIME | grep -v "Query timed out"

sleep 3600
