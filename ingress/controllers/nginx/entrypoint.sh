#!/bin/bash

set -eo pipefail

[[ $DEBUG ]] && set -x

DEF_SO_MAX_CONN=16384
DEF_LOCAL_PORT_RANGE="1024 65535"

LOCAL_PORT_RANGE="${LOCAL_PORT_RANGE:-DEF_LOCAL_PORT_RANGE}"
SO_MAX_CONN="${SO_MAX_CONN:-DEF_DEF_SO_MAX_CONN}"

# change values only if /writable-proc is mounted
if [ -d "/writable-proc" ]; then
  echo ${SO_MAX_CONN} > /writable-proc/sys/net/core/somaxconn
  echo ${LOCAL_PORT_RANGE} > /writable-proc/sys/net/ipv4/ip_local_port_range
fi

exec "$@"
