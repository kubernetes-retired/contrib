#!/bin/bash
# Copyright 2016 The Kubernetes Authors.
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
if [ ! $# -eq 5 ]; then
    echo "usage: construct.sh source_dir source_url source_branch destination_dir relative_destination_dir, source_dir and destination_dir are expected to be absolute paths."
    exit 1
fi
SRC="${1}"
SRCURL="${2}"
SRCBRANCH="${3}"
DST="${4}"
DSTRELATIVE="${5}"
pushd "${SRC}" > /dev/null
# checkout source branch
git checkout "${SRCBRANCH}"
# get the latest commit hash of source
commit_hash=$(git rev-parse HEAD)
popd > /dev/null
pushd "${DST}" > /dev/null
rm -rf ./*
cp -a "${SRC}/." "${DST}"
# move _vendor/ to vendor/
find "${DST}" -depth -name "_vendor" -type d -execdir mv {} "vendor" \;

commit_message="
Directory ${DSTRELATIVE} is copied from
${SRCURL}, branch ${SRCBRANCH},
last commit is ${commit_hash}"

echo "commit_message:${commit_message}"
