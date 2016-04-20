#!/bin/sh

if [ -z ${VAULT_SKIP_VERIFY} ]; then
  export VAULT_SKIP_VERIFY="true"
fi

if [ -z ${VAULT_TOKEN_FILE} ]; then
  export VAULT_TOKEN_FILE="/etc/vault-token/ingress-read-only"
fi

export VAULT_SKIP_VERIFY=true

if [ -z ${VAULT_TOKEN} ]; then
  [ -f ${VAULT_TOKEN_FILE} ] && export VAULT_TOKEN=`cat ${VAULT_TOKEN_FILE}`
fi

/controller