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

set -o nounset
set -o errexit
set -o pipefail

if [[ $# == 0 ]]; then
  echo >&2 "USAGE: $0 POD_NAME [POD_NAMESPACE CONTAINER_NAME]"
  exit 1
fi

# Customizable parameters
TMP_SUBDIR="${TMP_SUBDIR:-debug-tools}"
CONTEXT="${CONTEXT:-}"
DEBUGGER_NAME="${DEBUGGER_NAME:-debugger}"

# Name & namespace of target pod
NAME=$1
NAMESPACE=${2:-}
CONTAINER_NAME=${3:-}

# Internal variables
KUBECTL="kubectl"
[ -z "$CONTEXT" ] || KUBECTL="$KUBECTL --context=$CONTEXT"
[ -z "$NAMESPACE" ] || KUBECTL="$KUBECTL --namespace=$NAMESPACE"

TARGET_CNAME=${CONTAINER_NAME}
if [ -z "$CONTAINER_NAME" ]; then
  CONTAINER_NAME=$(${KUBECTL} get pod $NAME -o jsonpath='{.spec.containers[0].name}')
fi

INSTALL_DIR="/tmp/${TMP_SUBDIR}"
CONTAINER_ID=$(${KUBECTL} get pod $NAME -o jsonpath="{.status.containerStatuses[?(@.name==\"$CONTAINER_NAME\")].containerID}")
RUNTIME=${CONTAINER_ID%://*}
CONTAINER_ID=${CONTAINER_ID#*://}
NODE=$(${KUBECTL} get pod $NAME -o jsonpath='{.spec.nodeName}')

cat <<EOF
Debug Target Container:
  Pod:          $NAME
  Namespace:    ${NAMESPACE:-default}
  Node:         $NODE
  Container:    $CONTAINER_NAME
  Container ID: $CONTAINER_ID
  Runtime:      $RUNTIME

EOF

if [[ $RUNTIME != docker ]]; then
  echo >&2 "Error: $0 only works with a docker runtime. Found: $CONTAINER_ID"
  exit 1
fi

# Construct the command to debug the target container.
DEBUG_CMD="${KUBECTL} exec -i -t ${NAME}"
[ -z "$TARGET_CNAME" ] || DEBUG_CMD="$DEBUG_CMD -c $TARGET_CNAME"
DEBUG_CMD="$DEBUG_CMD -- ${INSTALL_DIR}/sh -c 'PATH=\$PATH:${INSTALL_DIR} sh'"

if $KUBECTL exec ${NAME} -c $CONTAINER_NAME -- ${INSTALL_DIR}/echo &>/dev/null; then
  echo "Debug tools already installed. Dumping you into the pod container now."
  eval "$DEBUG_CMD"
  exit 0
fi

echo "Installing busybox to $INSTALL_DIR ..."

# Cleanup the debugger pod.
function cleanup() {
  if ${KUBECTL} get pod ${DEBUGGER_NAME} &>/dev/null; then
    ${KUBECTL} delete pod ${DEBUGGER_NAME}
  fi
}

# If the debugger is not already running...
if ! ${KUBECTL} get pod ${DEBUGGER_NAME} &>/dev/null; then
  # Start the debug pod
  cat ${BASH_SOURCE[0]%/*}/debugger.yaml | \
    sed "s|ARG_DEBUGGER|${DEBUGGER_NAME}|" | \
    sed "s|ARG_NODENAME|${NODE}|" | \
    sed "s|ARG_NAMESPACE|${NAMESPACE}|" | \
    ${KUBECTL} create -f -
  trap cleanup EXIT
fi

# Wait for the pod to start
while [[ $(${KUBECTL} get pod ${DEBUGGER_NAME} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do
  echo "waiting for debugger pod to become ready..."
  sleep 1
done

# Call docker to copy busybox into the target container, and "install" it.
DOCKERCMD="/mnt/rootfs/usr/bin/docker -H unix:///run/docker.sock"
${KUBECTL} exec ${DEBUGGER_NAME} -- sh -c \
"mkdir -p ${INSTALL_DIR} && \
${DOCKERCMD} cp /tmp ${CONTAINER_ID}:/ && \
${DOCKERCMD} cp /bin/busybox ${CONTAINER_ID}:${INSTALL_DIR}/busybox && \
${DOCKERCMD} exec ${CONTAINER_ID} ${INSTALL_DIR}/busybox --install -s ${INSTALL_DIR}"

echo "Installation complete."

echo "To debug $NAME, run:
    $DEBUG_CMD
Dumping you into the pod container now.
"

eval "$DEBUG_CMD"
