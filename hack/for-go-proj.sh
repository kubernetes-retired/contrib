#! /bin/bash

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
set -o pipefail
set -o nounset

CONTRIB_ROOT="$(dirname ${BASH_SOURCE})/.."

godep_projects=$(find "${CONTRIB_ROOT}" -wholename '*Godeps/Godeps.json')

CMD="${1}"

case "${CMD}" in
  "build")
    ;;
  "install")
    ;;
  "test")
    ;;
  *)
    echo "invalid subcommand: ${CMD}"
    exit 1
    ;;
esac

for godep_file in ${godep_projects}; do
  (
    project="${godep_file%Godeps/Godeps.json}"
    echo "${CMD}ing ${project}"
    cd "${project}"
    godep go "${CMD}" ./...
  )
done;
