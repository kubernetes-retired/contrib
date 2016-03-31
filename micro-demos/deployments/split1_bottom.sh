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

. $(dirname ${BASH_SOURCE})/../util.sh

target="$1"

notFound=true
rsName=""
while [ "${notFound}" = true ]; do
  for ((i=0;i<=1;i++)); do
    rs=$(kubectl --namespace=demos get rs -o go-template="{{(index .items $i).metadata.name}}" 2>/dev/null)
    contains=$(kubectl --namespace=demos get rs "${rs}" -o go-template='{{.spec.template.spec.containers}}' 2>/dev/null | grep "${target}")
    if [ ! -z "${contains}" ]; then
      notFound=false
      rsName="$rs"
    fi
  done
done

trap "exit" INT
while true; do
  kubectl --namespace=demos get rs "${rsName}" 2>/dev/null | awk -v var="$target" '{if (NR==2) {print "Desired replicas of "var" ReplicaSet: " $2 " Running: " $3}}'
done
