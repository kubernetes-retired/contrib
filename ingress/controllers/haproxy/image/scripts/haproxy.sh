#!/bin/sh

# Copyright 2015 The Kubernetes Authors.
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

set -e

HAPROXY_PID=/var/run/haproxy.pid

haproxy_start()
{
  haproxy -p $HAPROXY_PID -D -f $BALANCER_CFG || return 2
}

haproxy_stop()
{
  if [ ! -f $HAPROXY_PID ] ; then
    # assume no process is running
    return 0
  fi
  for pid in $(cat $HAPROXY_PID) ; do
    /bin/kill -9 $pid || return 2
  done
  rm -f $HAPROXY_PID
  return 0
}

haproxy_reload()
{
  haproxy -p $HAPROXY_PID -D -f $BALANCER_CFG -sf $(cat $HAPROXY_PID)
}


case "$1" in
start)
  haproxy_start
  ;;
stop)
  haproxy_stop
  ;;
reload)
  haproxy_reload
  ;;
*)
  echo "Supported commands are: start, stop, reload"
  exit 2
esac
