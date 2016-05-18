#! /bin/bash

# This script configures zookeeper cluster member ship for version of zookeeper
# >= 3.5.0. It should not be used with the on-change.sh script in this example.
# As of April-2016 is 3.4.8 is the latest stable.

CFG=/opt/zookeeper/conf/zoo.cfg.dynamic
CFG_BAK=/opt/zookeeper/conf/zoo.cfg.bak
MY_ID_FILE=/tmp/zookeeper/myid

# write myid
IFS='-' read -ra ADDR <<< "$(hostname)"
MY_ID=$(expr "1" + "${ADDR[1]}")
echo $MY_ID > "${MY_ID_FILE}"

while read -ra LINE; do
    PEERS=("${PEERS[@]}" $LINE)
done

# Don't add the first member as an observer
if [ ${#PEERS[@]} -eq 1 ]; then
    echo "server.1=${PEERS[0]}:2888:3888;2181" > "${CFG}"
    /opt/zookeeper/bin/zkServer-initialize.sh --force --myid=$MY_ID
    /opt/zookeeper/bin/zkServer.sh start
    exit
fi

# Every subsequent member is added as an observer and promoted to a participant
echo "" > "${CFG_BAK}"
i=0
for peer in "${PEERS[@]}"; do
    let i=i+1
    if [ $i = $MY_ID ]; then
      echo "server.${i}=${peer}:2888:3888:observer;2181" >> "${CFG_BAK}"
      MY_NAME=${peer}
    else
      echo "server.${i}=${peer}:2888:3888:participant;2181" >> "${CFG_BAK}"
    fi
done

# Once the dynamic config file is written it shouldn't be modified, so the final
# reconfigure needs to happen through the "reconfig" command.
cp ${CFG_BAK} ${CFG}
/opt/zookeeper/bin/zkServer-initialize.sh --force --myid=$MY_ID
/opt/zookeeper/bin/zkServer.sh start

# TODO: We shouldn't need to specify the address of the master as long as
# there's quorum. According to the docs the new server is just not allowed to
# vote, it's still allowed to propose config changes, and it knows the
# existing members of the ensemble from *its* config. This works as expected,
# but we should correlate with more satisfying empirical evidence.
/opt/zookeeper/bin/zkCli.sh reconfig -add "server.$MY_ID=$MY_NAME:2888:3888:participant;2181"
/opt/zookeeper/bin/zkServer.sh restart

