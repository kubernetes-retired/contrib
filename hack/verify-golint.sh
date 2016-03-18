#!/bin/bash

# Copyright 2014 The Kubernetes Authors All rights reserved.
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

KUBE_ROOT=$(dirname "${BASH_SOURCE}")/..

GO_VERSION=($(go version))
# golint only works for golang 1.5+
if [[ -n $(echo "${GO_VERSION[2]}" | grep -E 'go1.1|go1.2|go1.3|go1.4') ]]; then
  echo "GOLINT requires go 1.5+. Skipping"
  exit
fi

cd "${KUBE_ROOT}"

GOLINT=${GOLINT:-"golint"}
bad_files=$($GOLINT -min_confidence=0.9 ./...)
if [[ -n "${bad_files}" ]]; then
  echo "!!! '$GOLINT' problems: "
  echo "${bad_files}"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
