#!/bin/sh

if [ -z ${VAULT_SKIP_VERIFY} ]; then
  export VAULT_SKIP_VERIFY="true"
fi

if [ -z ${VAULT_TOKEN_FILE} ]; then
  export VAULT_TOKEN_FILE="/etc/vault-token/vault-ingress-read-only"
fi

export VAULT_SKIP_VERIFY=true

if [ -z ${VAULT_TOKEN} ]; then
  [ -f ${VAULT_TOKEN_FILE} ] && export VAULT_TOKEN=`cat ${VAULT_TOKEN_FILE}`
fi

if [ -z ${DEBUG} ]; then
  export DEBUG=false
fi

if [ ! -z "${VAULT_SSL_SIGNER}" ]; then
  echo "${VAULT_SSL_SIGNER}" | sed -e 's/\"//g' | sed -e 's/^[ \t]*//g' | sed -e 's/[ \t]$//g' >> /etc/ssl/certs/ca-certificates.crt
fi

openssl req -x509 -newkey rsa:2048 -nodes -keyout /etc/nginx/certs/localhost.key -out /etc/nginx/certs/localhost.crt -days 365 -subj "/CN=localhost" 

if [ ${DEBUG} = "true" ]; then
  cat /etc/nginx/nginx.conf
  ls -l /etc/nginx/certs
  ls -l /etc/nginx/conf.d
  ps -ef | grep nginx | grep -v grep
  ps -efH
  netstat -an | grep LISTEN
fi

/bin/bash -c /controller