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

if kubectl --namespace=demos get rc hostnames >/dev/null 2>&1; then
    desc "Revisit our replication controller"
    run "kubectl --namespace=demos get rc hostnames"
else
    desc "Run some pods under a replication controller"
    run "kubectl --namespace=demos run hostnames \\
        --image=gcr.io/google_containers/serve_hostname:1.1 --replicas=5"
fi

desc "Expose the RC as a service"
run "kubectl --namespace=demos expose rc hostnames \\
    --port=80 --target-port=9376"

desc "Have a look at the service"
run "kubectl --namespace=demos describe svc hostnames"

IP=$(kubectl --namespace=demos get svc hostnames \
    -o go-template='{{.spec.clusterIP}}')
desc "See what happens when you access the service's IP"
run "gcloud compute ssh --zone=us-central1-b $SSH_NODE --command '\\
    for i in \$(seq 1 10); do \\
        curl --connect-timeout 1 -s $IP && echo; \\
    done \\
    '"
run "gcloud compute ssh --zone=us-central1-b $SSH_NODE --command '\\
    for i in \$(seq 1 500); do \\
        curl --connect-timeout 1 -s $IP && echo; \\
    done | sort | uniq -c; \\
    '"

tmux new -d -s my-session \
    "$(dirname ${BASH_SOURCE})/split1_lhs.sh" \; \
    split-window -h -d "sleep 10; $(dirname $BASH_SOURCE)/split1_rhs.sh" \; \
    attach \;
