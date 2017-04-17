#! /bin/bash

# This test is for dev purposes. It reads like golang, deal with it.

set -e
source ../../../hack/testlib.sh
APP=${APP:-haproxyhttps}
PUSH=${PUSH:-false}
HOSTS=${HOST:-haproxyhttps}

function setup {
    cleanup "${APP}"
    makeCerts ${APP} ${HOSTS[*]}
    if "${PUSH}"; then
        make push
    fi
    "${K}" create -f haproxy-https.yaml
    waitForPods "${APP}"
}

function run {
    frontendIP=`getNodeIPs frontend`
    echo Frontend ip ${frontendIP[*]}

    for h in ${HOSTS[*]}; do
        for ip in ${frontendIP[*]}; do
            curlHTTPSWithHost $h 443 $ip $h.crt
            # This will just redirect to https
            # curlNodePort "${APP}"
        done
    done
    cleanup "${APP}"
}

setup
run

