#!/bin/bash
# Copyright 2017 The Kubernetes Authors.
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

desc "Create a Pod with init containers"
run "cat $(relative pod.yaml)"
run "kubectl -n demos create -f $(relative pod.yaml)"

desc "See what happened"
run "kubectl -n demos exec -ti init-ctr-demo -c busybox cat /data/file"

desc "Clean up"
run "kubectl -n demos delete pod init-ctr-demo"
