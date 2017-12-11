#!/bin/sh
source staging-variables.sh && \
./generate-staging-config.py $NUM_MASTERS $NUM_WORKERS $AWS_REGION $PUBLIC_IP $DNS_ADDRESS \
$AWS_ACCESS_KEY_ID $AWS_SECRET_ACCESS_KEY "$AWS_KEY_NAME" && \
terraform apply
