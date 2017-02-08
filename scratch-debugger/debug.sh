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
CONTEXT="${KUBECONTEXT:-}"
DEBUGGER_NAME="${DEBUGGER_NAME:-debugger}"
ARCH="${ARCH:-amd64}"

# Name & namespace of target pod
NAME=$1
NAMESPACE=${2:-}
CONTAINER_NAME=${3:-}

# Internal variables
KUBECTL="kubectl"
[ -z "$CONTEXT" ] || KUBECTL="$KUBECTL --context=$CONTEXT"
[ -z "$NAMESPACE" ] || KUBECTL="$KUBECTL --namespace=$NAMESPACE"

NAMESPACE=${NAMESPACE:-default}

TARGET_CNAME=${CONTAINER_NAME}
if [ -z "$CONTAINER_NAME" ]; then
  CONTAINER_NAME=$(${KUBECTL} get pod $NAME -o jsonpath='{.spec.containers[0].name}')
fi

INSTALL_DIR="/tmp/${TMP_SUBDIR}"
CONTAINER_ID=$(${KUBECTL} get pod $NAME -o jsonpath="{.status.containerStatuses[?(@.name==\"$CONTAINER_NAME\")].containerID}")
RUNTIME=${CONTAINER_ID%://*}
CONTAINER_ID=${CONTAINER_ID#*://}
NODE=$(${KUBECTL} get pod $NAME -o jsonpath='{.spec.nodeName}')

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

cat <<EOF
Debug Target Container:
  Pod:          $NAME
  Namespace:    $NAMESPACE
  Node:         $NODE
  Container:    $CONTAINER_NAME
  Container ID: $CONTAINER_ID
  Runtime:      $RUNTIME

  "Installing busybox to $INSTALL_DIR ..."
EOF

# Cleanup the debugger pod.
function cleanup() {
  if ${KUBECTL} get pod ${DEBUGGER_NAME} &>/dev/null; then
    ${KUBECTL} delete pod ${DEBUGGER_NAME}
  fi
}

case $ARCH in
  "amd64")
    IMAGE=busybox
    ;;
  "arm")
    IMAGE=armhf/busybox
    ;;
  "arm64")
    IMAGE=aarch64/busybox
    ;;
  "ppc64le")
    IMAGE=ppc64le/busybox
    ;;
  "s390x")
    IMAGE=s390x/busybox
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
esac

if  ${KUBECTL} get pod ${DEBUGGER_NAME} &>/dev/null; then
  echo 2> "Error: pod ${DEBUGGER_NAME} is already running in namespace ${NAMESPACE}"
  exit 1
fi

DOCKERCMD="/mnt/rootfs/usr/bin/docker -H unix:///run/docker.sock"

${KUBECTL} create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name:        ${DEBUGGER_NAME}
  namespace:   ${NAMESPACE}
spec:
  nodeName:    ${NODE}
  restartPolicy: Never
  containers:
    - name:    debugger
      image:   ${IMAGE}
      securityContext:
        privileged: true
      command:
        - sh
        - -c
        - "set -x; mkdir -p ${INSTALL_DIR} && \
           ${DOCKERCMD} cp /tmp ${CONTAINER_ID}:/ && \
           ${DOCKERCMD} cp /bin/busybox ${CONTAINER_ID}:${INSTALL_DIR}/busybox && \
           ${DOCKERCMD} exec ${CONTAINER_ID} ${INSTALL_DIR}/busybox --install -s ${INSTALL_DIR}"
      # Mount the node FS for direct access to docker.
      volumeMounts:
        - name: rootfs
          mountPath: /mnt/rootfs
          readOnly: true
        - name: rootfs-run
          mountPath: /mnt/rootfs/var/run
          readOnly: true
  volumes:
    - name: rootfs
      hostPath:
        path: /
    - name: rootfs-run
      hostPath:
        path: /var/run
EOF
trap cleanup EXIT

# Wait for the pod to terminate.
PHASE=$(${KUBECTL} get pod -a ${DEBUGGER_NAME} -o jsonpath='{.status.phase}')
while [[ ! $PHASE =~ (Succeeded|Failed) ]]; do
  echo "waiting for debugger pod to complete (currently $PHASE)..."
  sleep 1
  PHASE=$(${KUBECTL} get pod -a ${DEBUGGER_NAME} -o jsonpath='{.status.phase}')
done
if [[ $PHASE == "Failed" ]]; then
  echo 2> "Pod failed:"
  ${KUBECTL} logs ${DEBUGGER_NAME}
  exit 1
fi

cleanup

echo "Installation complete."

echo "To debug $NAME, run:
    $DEBUG_CMD
Dumping you into the pod container now.
"

eval "$DEBUG_CMD"
