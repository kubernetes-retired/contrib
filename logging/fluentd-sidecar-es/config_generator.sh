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

mkdir -p /etc/td-agent/files
if [ -z "$FILES_TO_COLLECT" ]; then
  exit 0
fi

for filepath in $FILES_TO_COLLECT
do
  filename=$(basename $filepath)
  cat > "/etc/td-agent/files/${filename}" << EndOfMessage
<source>
  type tail
  format none
  time_key time
  path ${filepath}
  pos_file /etc/td-agent/fluentd-es.log.pos
  time_format %Y-%m-%dT%H:%M:%S
  tag file.${filename}
  read_from_head true
</source>
EndOfMessage
done

if [ -z "$FILES_TO_ROTATE" ] || [ -z "$SIZE_LIMIT" ] || [ -z "$ROTATE_TIMES" ]; then
  eixt 0
fi
read -ra files_to_rotate <<<"$FILES_TO_ROTATE"
read -ra size_limit <<<"$SIZE_LIMIT"
read -ra rotate_times <<<"$ROTATE_TIMES"

if [ ${#files_to_rotate[@]} -eq ${#size_limit[@]} ] && [ ${#files_to_rotate[@]} -eq ${#rotate_times[@]} ]; then
  for i in ${!files_to_rotate[*]}
  do
    cat >> "/etc/logrotate.conf" << EndOfMessage
${files_to_rotate[i]} {
  size ${size_limit[i]}
  rotate ${rotate_times[i]}
}
EndOfMessage
  done
fi