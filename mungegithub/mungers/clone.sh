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

set -o errexit
set -o nounset
set -o pipefail

echo $@
if [ ! $# -eq 2 ]; then
    echo "usage: clone.sh destination_dir destination_url, destination_dir is expected to be absolute paths."
    exit 1
fi

DST="${1}"
DSTURL="${2}"
# set up the destination directory
rm -rf "${DST}"
mkdir -p "${DST}"
git clone "${DSTURL}" "${DST}"
