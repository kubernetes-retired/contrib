#!/bin/bash

# Copyright 2014 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# Export proxy to ensure commands like curl could work
[[ -n "${HTTP_PROXY:-}" ]]  && export HTTP_PROXY=${HTTP_PROXY}
[[ -n "${HTTPS_PROXY:-}" ]] && export HTTPS_PROXY=${HTTPS_PROXY}

# Caller should set in the ev:
# MASTER_IP - this may be an ip or things like "_use_gce_external_ip_"
# MASTERS - DNS name for the masters
# DNS_DOMAIN - which will be passed to minions in --cluster-domain
# SERVICE_CLUSTER_IP_RANGE - where all service IPs are allocated
# KUBE_CERT_KEEP_CA - to keep ca.key file or not, "true"|"false"

# Also the following will be respected
# CERT_DIR - where to place the finished certs
# CERT_GROUP - who the group owner of the cert files should be

cert_ip="${MASTER_IP:="${1}"}"
masters="${MASTERS:="kubernetes"}"
service_range="${SERVICE_CLUSTER_IP_RANGE:="10.0.0.0/16"}"
dns_domain="${DNS_DOMAIN:="cluster.local"}"
cert_dir="${CERT_DIR:-"/srv/kubernetes"}"
cert_group="${CERT_GROUP:="kube-cert"}"
keep_ca_priv_key="${KUBE_CERT_KEEP_CA:="false"}"

# The following certificate pairs are created:
#
#  - ca (the cluster's certificate authority)
#  - server
#  - kubelet
#  - kubecfg (for kubectl)
#
# TODO(roberthbailey): Replace easyrsa with a simple Go program to generate
# the certs that we need.

# TODO: Add support for discovery on other providers?
if [ "$cert_ip" == "_use_gce_external_ip_" ]; then
  cert_ip=$(curl -s -H Metadata-Flavor:Google http://metadata.google.internal./computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip)
  if test -z "${cert_ip}"; then
      echo "Failed to retrieve external IP" 1>&2; exit 1
  fi
fi

if [ "$cert_ip" == "_use_aws_external_ip_" ]; then
  cert_ip=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
  if test -z "${cert_ip}"; then
      echo "Failed to retrieve external IP" 1>&2; exit 1
  fi
fi

if [ "$cert_ip" == "_use_azure_dns_name_" ]; then
  cert_ip=$(uname -n | awk -F. '{ print $2 }').cloudapp.net
  if test -z "${cert_ip}"; then
      echo "Failed to retrieve external IP" 1>&2; exit 1
  fi
fi

tmpdir=$(mktemp -d --tmpdir kubernetes_cacert.XXXXXX)
trap 'rm -rf "${tmpdir}"' EXIT
cd "${tmpdir}"

# TODO: For now, this is a patched tool that makes subject-alt-name work, when
# the fix is upstream  move back to the upstream easyrsa.  This is cached in GCS
# but is originally taken from:
#   https://github.com/brendandburns/easy-rsa/archive/master.tar.gz
#
# To update, do the following:
# curl -o easy-rsa.tar.gz https://github.com/brendandburns/easy-rsa/archive/master.tar.gz
# gsutil cp easy-rsa.tar.gz gs://kubernetes-release/easy-rsa/easy-rsa.tar.gz
# gsutil acl ch -R -g all:R gs://kubernetes-release/easy-rsa/easy-rsa.tar.gz
#
# Due to GCS caching of public objects, it may take time for this to be widely
# distributed.

# Calculate the first ip address in the service range
octets=($(echo "${service_range}" | sed -e 's|/.*||' -e 's/\./ /g'))
((octets[3]+=1))
service_ip=$(echo "${octets[*]}" | sed 's/ /./g')

# Determine appropriete subject alt names
declare -a san_array=(IP:${service_ip} DNS:kubernetes DNS:kubernetes.default DNS:kubernetes.default.svc DNS:kubernetes.default.svc.${dns_domain})

IFS=',' read -ra cert_ip <<< "$cert_ip"
for ip in "${cert_ip[@]}"; do
    san_array+=(IP:${ip})
done

IFS=',' read -ra masters <<< "$masters"
for master in "${masters[@]}"; do
    san_array+=(DNS:${master})
done

if [[ -n "${CLUSTER_HOSTNAME}" ]]; then
    san_array+=(DNS:${CLUSTER_HOSTNAME})
fi

if [[ -n "${CLUSTER_PUBLIC_HOSTNAME}" ]]; then
    san_array+=(DNS:${CLUSTER_PUBLIC_HOSTNAME})
fi

sans="$(IFS=, ; echo "${san_array[*]}")"

curl -sSL -O https://storage.googleapis.com/kubernetes-release/easy-rsa/easy-rsa.tar.gz
tar xzf easy-rsa.tar.gz
cd easy-rsa-master/easyrsa3

# Sadly, openssl is very verbose to std*err* with no option to turn it off.
if ! (./easyrsa --batch init-pki
      # Since the length of CN is limited to 64 bytes, here we cut too long ${cert_ip}
      ./easyrsa --batch "--req-cn=$(echo ${cert_ip} | cut -b 1-$(expr 64 - $(echo @$(date +%s) | wc -c)))@$(date +%s)" build-ca nopass
      ./easyrsa --batch --subject-alt-name="${sans}" build-server-full master nopass
      ./easyrsa --batch build-client-full kubelet nopass
      ./easyrsa --batch build-client-full kubecfg nopass) >/dev/null 2>&1; then
    echo "=== Failed to generate certificates: Aborting ===" 1>&2
    exit 2
fi

mkdir -p "$cert_dir"

cp -p pki/ca.crt "${cert_dir}/ca.crt"
cp -p pki/issued/master.crt "${cert_dir}/server.crt"
cp -p pki/private/master.key "${cert_dir}/server.key"
cp -p pki/issued/kubecfg.crt "${cert_dir}/kubecfg.crt"
cp -p pki/private/kubecfg.key "${cert_dir}/kubecfg.key"
cp -p pki/issued/kubelet.crt "${cert_dir}/kubelet.crt"
cp -p pki/private/kubelet.key "${cert_dir}/kubelet.key"

CERTS=("ca.crt" "server.key" "server.crt" "kubelet.key" "kubelet.crt" "kubecfg.key" "kubecfg.crt")
if [[ "${keep_ca_priv_key}" == "true" ]]; then
    cp -p pki/private/ca.key "${cert_dir}/ca.key"
    CERTS+=("ca.key")
fi
for cert in "${CERTS[@]}"; do
  chgrp "${cert_group}" "${cert_dir}/${cert}"
  chmod 660 "${cert_dir}/${cert}"
done
