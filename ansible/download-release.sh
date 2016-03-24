#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

mkdir -p binaries

SCRIPT_URL=https://raw.githubusercontent.com/kubernetes/kubernetes/master/cluster/ubuntu/download-release.sh

curl $SCRIPT_URL | bash
