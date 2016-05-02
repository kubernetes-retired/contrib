#! /bin/bash

# This script configures zookeeper cluster member ship for version of zookeeper
# < 3.5.0. It should not be used with the on-start.sh script in this example.
# As of April-2016 is 3.4.8 is the latest stable.

CFG=/opt/zookeeper/conf/zoo.cfg
CFG_BAK=/opt/zookeeper/conf/zoo.cfg.bak
MY_ID=/tmp/zookeeper/myid

# write myid
IFS='-' read -ra ADDR <<< "$(hostname)"
echo $(expr "1" + "${ADDR[1]}") > "${MY_ID}"

# TODO: This is a dumb way to reconfigure zookeeper because it allows dynamic
# reconfig, but it's simple.
i=0
echo "
tickTime=2000
initLimit=10
syncLimit=5
dataDir=/tmp/zookeeper
clientPort=2181
" > "${CFG_BAK}"

while read -ra LINE; do
    let i=i+1
    echo "server.${i}=${LINE}:2888:3888" >> "${CFG_BAK}"
done
cp ${CFG_BAK} ${CFG}

# TODO: Typically one needs to first add a new member as an "observer" then
# promote it to "participant", but that requirement is relaxed if we never
# start > 1 at a time.
/opt/zookeeper/bin/zkServer.sh restart
