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

# We need the salt-master to be up for the minions to work
sudo journalctl -f /usr/bin/cloud-init & 
attempt=0
while true; do
  echo -n Attempt "$(($attempt+1))" to check for salt-master
  ok=1
  output=$(pgrep salt-master) || ok=0
  if [[ ${ok} == 0 ]]; then
    if (( attempt > 30 )); then
      echo
      echo "(Failed) output was: ${output}"
      echo
      echo "salt-master failed to start"
      exit 1
    fi
  else
    echo "[salt-master running]"
    break
  fi
  echo " [salt-master not working yet]"
  attempt=$(($attempt+1))
  sleep 10
done
