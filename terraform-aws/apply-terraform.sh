#!/bin/sh
source aws-variables.sh && \
./master/make-cloud-config.py && \
./worker/make-cloud-config.py && \
terraform apply -var "access_key=$AWS_ACCESS_KEY_ID" -var "secret_key=$AWS_SECRET_ACCESS_KEY" \
-var "region=$AWS_REGION" -var "master_ip=$AWS_MASTER_IP" -var "worker_ip=$AWS_WORKER_IP" \
-var "aws_key_name=$AWS_KEY_NAME" -var "dns_address=$DNS_ADDRESS"
