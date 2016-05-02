#! /bin/bash

# This script configures zookeeper cluster member ship for version of zookeeper
# < 3.5.0. It should not be used with the on-start.sh script in this example.
# As of April-2016 is 3.4.8 is the latest stable.

CFG=/etc/mysql/my.cnf

function join {
    local IFS="$1"; shift; echo "$*";
}

# Parse out hostname, formatted as: petset_name-index
IFS='-' read -ra ADDR <<< "$(hostname)"
MY_ID=$(expr "1" + "${ADDR[1]}")
CLUSTER_NAME="${ADDR[0]}"

i=0
while read -ra LINE; do
    let i=i+1
    if [ $i = $MY_ID ]; then
        MY_NAME=$LINE
    fi
    PEERS=("${PEERS[@]}" $LINE)
done
if [ "${#PEERS[@]}" = 1 ]; then
    WSREP_CLUSTER_ADDRESS=""
else
    WSREP_CLUSTER_ADDRESS=$(join , "${PEERS[@]}")
fi
sed -i -e "s|^wsrep_node_address=.*$|wsrep_node_address=${MY_NAME}|" ${CFG}
sed -i -e "s|^wsrep_cluster_name=.*$|wsrep_cluster_name=${CLUSTER_NAME}|" ${CFG}
sed -i -e "s|^wsrep_cluster_address=.*$|wsrep_cluster_address=gcomm://${WSREP_CLUSTER_ADDRESS}|" ${CFG}

# don't need a restart, we're just writing the conf in case there's an
# unexpected restart on the node.
