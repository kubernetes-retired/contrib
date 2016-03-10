#!/usr/bin/env python3
import subprocess
import urllib.request
import os.path


os.chdir(os.path.abspath(os.path.dirname(__file__)))


with urllib.request.urlopen('https://discovery.etcd.io/new?size=1') \
        as response:
    discovery_url = response.read().decode()

with open('./cloud-config', 'wt') as fcloud_config:
    fcloud_config.write("""#cloud-config

users:
coreos:
  etcd2:
    discovery: {0}
    advertise-client-urls: https://{1}:2379
    initial-advertise-peer-urls: https://{1}:2380
    listen-client-urls: https://0.0.0.0:2379
    listen-peer-urls: https://{1}:2380
  units:
    - name: etcd2.service
      command: start
      drop-ins:
        - name: 30-certificates.conf
          content: |
            [Service]
            # Client Env Vars
            Environment=ETCD_TRUSTED_CA_FILE=/etc/kubernetes/ssl/ca.pem
            Environment=ETCD_CERT_FILE=/etc/kubernetes/ssl/master1-master-client.pem
            Environment=ETCD_KEY_FILE=/etc/kubernetes/ssl/master1-master-client-key.pem
            # Peer Env Vars
            Environment=ETCD_PEER_TRUSTED_CA_FILE=/etc/kubernetes/ssl/ca.pem
            Environment=ETCD_PEER_CERT_FILE=/etc/kubernetes/ssl/master1-master-peer.pem
            Environment=ETCD_PEER_KEY_FILE=/etc/kubernetes/ssl/master1-master-peer-key.pem
    - name: fleet.service
      command: start
""".format(discovery_url, '172.31.29.111'))
