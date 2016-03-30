#!/bin/sh
source staging-variables.sh && \
./generate-staging-config.py $NUM_WORKERS $PROJECT $REGION $ZONE $PUBLIC_IP $PUBLIC_IP \
$DNS_ADDRESS && terraform apply
