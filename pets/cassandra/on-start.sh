#!/bin/bash
set -e

# Parse out hostname, formatted as: petset_name-index
IFS='-' read -ra ADDR <<< "$(hostname)"
MY_ID=$(expr "1" + "${ADDR[1]}")
CFG=/etc/cassandra/cassandra.yaml

i=0
while read -ra LINE; do
    let i=i+1
    if [ $i = $MY_ID ]; then
        MY_NAME=$LINE
    fi
    PEERS=("${PEERS[@]}" $LINE)
done

CASSANDRA_RPC_ADDRESS='0.0.0.0'
# TODO: This is the default, and is the number of tokens assigned to this node
# on the ring. There's probably some optimization possible here.
CASSANDRA_NUM_TOKENS=256

CASSANDRA_CLUSTER_NAME="${ADDR[0]}"
CASSANDRA_LISTEN_ADDRESS="$MY_NAME"
CASSANDRA_BROADCAST_ADDRESS="$MY_NAME"
CASSANDRA_BROADCAST_RPC_ADDRESS="$MY_NAME"

# TODO: Very first node is seed, make this configurable upto 3
sed -ri 's/(- seeds:) "127.0.0.1"/\1 "'"${PEERS[0]}"'"/' "$CFG"

for yaml in \
    broadcast_address \
	broadcast_rpc_address \
	cluster_name \
	listen_address \
	num_tokens \
	rpc_address \
; do
    var="CASSANDRA_${yaml^^}"
	val="${!var}"
	if [ "$val" ]; then
		sed -ri 's/^(# )?('"$yaml"':).*/\2 '"$val"'/' "$CFG"
	fi
done

# TODO: Run as cassandra user and remove -R
cassandra -R
