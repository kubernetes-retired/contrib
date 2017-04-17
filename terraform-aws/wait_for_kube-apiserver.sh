#!/bin/bash
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

# We need the kubernetes apiserver to be up to finish provisioning
sudo journalctl -u docker &
attempt=0
while true; do
  echo -n Attempt "$(($attempt+1))" to check for kube-apiserver
  ok=1
  output=$(pgrep kube-apiserver) || ok=0
  if [[ ${ok} == 0 ]]; then
    if (( attempt > 30 )); then
      echo
      echo "(Failed) output was: ${output}"
      echo
      echo "kube-apiserver failed to start"
      exit 1
    fi
  else
    echo "[kube-apiserver running]"
    break
  fi
  echo " [kube-apiserver not working yet]"
  attempt=$(($attempt+1))
  sleep 10
done
