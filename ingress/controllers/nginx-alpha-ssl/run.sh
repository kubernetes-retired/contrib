#!/bin/sh

export VAULT_SKIP_VERIFY=true
[ -f /etc/vault-token/ingress-read-only ] && export VAULT_TOKEN=`cat /etc/vault-token/ingress-read-only`

/controller