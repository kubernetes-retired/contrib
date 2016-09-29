#!/bin/sh
# Authors: stackpoint.io

set -e



case "$1" in
start)
  echo "start"
  ;;
stop)
  echo "stop"
  ;;
reload)
  echo "reload"
  ;;
*)
  echo "Supported commands are: start, stop, reload"
  exit 2
esac
