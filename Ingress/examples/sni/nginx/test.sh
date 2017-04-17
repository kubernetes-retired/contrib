#! /bin/bash

# This test is only meant to illustrate a working example of an Nginx pod
# capable of SNI.

set -e
source ../../../hack/testlib.sh
# Name of the app in the .yaml
APP=${APP:-nginxsni}
# SNI hostnames
HOSTS=(wildcard nginx2 nginx3)
# Should the test build and push the container via make push?
PUSH=${PUSH:-false}

# setup set's up the environment for run
function setup {
    cleanup "${APP}"
    makeCerts ${APP} ${HOSTS[*]}
    if "${PUSH}"; then
        make push
    fi
    "${K}" create -f nginx-sni.yaml
    waitForPods "${APP}"
}

# run runs the test
function run {
    local frontendIP=`getNodeIPs frontend`
    echo Frontend ip ${frontendIP[*]}

    set +e
    for h in ${HOSTS[*]}; do
        for ip in ${frontendIP[*]}; do
            for i in 1 2 3 4 5; do
                cname=${h}
                # TODO: Just convert everything to .com
                if [ $cname == "wildcard" ]; then
                    cname="foo.wildcard.com"
                fi
                curlHTTPSWithHost "${cname}" 8082 "${ip}" "${h}".crt
                if [ $? == 0 ]; then
                    break
                else
                    sleep 1
                fi
            done
        done
    done
    set -e
    cleanup "${APP}"
}

setup
run
