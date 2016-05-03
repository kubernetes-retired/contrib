#!/bin/bash
set -e

# Parse out hostname, formatted as: petset_name-index
IFS='-' read -ra ADDR <<< "$(hostname)"
MY_ID=$(expr "1" + "${ADDR[1]}")
CFG=/etc/redis/redis.conf
PORT=6379

# It's nice to have an init script, but we never want to query ourselves in
# stand-alone mode and think we're a master.
sudo service redis-server stop
i=0
while read -ra LINE; do
    let i=i+1
    if [ $i = $MY_ID ]; then
        sed -i -e "s|^bind.*$|bind ${LINE}|" ${CFG}
    elif [ "$(redis-cli -h $LINE info | grep role | sed 's,\r$,,')" = "role:master" ]; then
        # TODO: More restrictive regex?
        sed -i -e "s|^.*slaveof.*$|slaveof ${LINE} ${PORT}|" ${CFG}
    fi
done
sudo service redis-server start
