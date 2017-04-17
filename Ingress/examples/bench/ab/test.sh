#! /bin/bash

# This test is only meant to document the behavior of the pod in this directory.

set -e
source ../../../hack/testlib.sh
APP=${APP:-nginxtest}

function setup {
    echo "${K}"
    checkCluster
    cleanup "${APP}"
    "${K}" create -f nginx-app.yaml
    "${K}" get rc,svc,pods -l app="${APP}"
    waitForPods "${APP}"
}

function run {
    set +e
    for i in 1 2 3 4 5; do
        curlNodePort "${APP}"
        if [ $? == 0 ]; then
            break
        fi
    done
    set -e
    cleanup "${APP}"
}

setup
run

