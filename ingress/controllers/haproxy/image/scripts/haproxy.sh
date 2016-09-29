#!/bin/sh
# Authors: stackpoint.io

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
