#!/usr/bin/env bash
# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# zkMetrics uses the mntr four letter word to retrieve the ZooKeeper 
# instances metrics and dump them to standard out. If the ZK_CLIENT_PORT 
# enviorment varibale is not set 2181 is used.

ZK_CLIENT_PORT=${ZK_CLIENT_PORT:-2181}
echo mntr | nc localhost $ZK_CLIENT_PORT >& 1
