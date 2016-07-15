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

# Sample usage: pr-generator /kubernetes 0fb323... "increase frequency of scalability-gce test"

set -o errexit
set -o nounset
set -o pipefail

repo=$1
user=$2
token=$3
pr_comment=$4
branch_name=$5

# Assumes that pwd is a repo
function assure_upstream {
  if [[ -z "$(git remote -v | grep upstream)" ]]; then
    git remote add upstream "https://github.com/kubernetes/kubernetes.git/master"
    git remote set-url --push upstream no_push
  fi
}

cd "${repo}"
if [[ -n "$(git status | grep 'nothing to commit, working directory clean')" ]]; then
	echo "No changes"
	exit 1
fi

git stash
git commit -a -m"${pr_comment}"
assure_upstream
git fetch upstream
git rebase upstream/master
git stash pop
git checkout -b "${branch_name}"
git push -f "https://${user}:${token}@github.com/${user}/kubernetes.git" "${branch_name}"
git checkout master
