#!/bin/sh
source staging-variables.sh && \
./generate-staging-config.py $NUM_MASTERS $NUM_WORKERS $PUBLIC_IP $DNS_ADDRESS && \
terraform apply
