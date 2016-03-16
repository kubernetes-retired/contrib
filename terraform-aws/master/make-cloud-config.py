#!/usr/bin/env python3
import subprocess
import urllib.request
import os.path


os.chdir(os.path.abspath(os.path.dirname(__file__)))


with urllib.request.urlopen('https://discovery.etcd.io/new?size=1') \
        as response:
    discovery_url = response.read().decode()

with open('./assets/cloud-config', 'wt') as fcloud_config:
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
            Environment=ETCD_CERT_FILE=/etc/kubernetes/ssl/master-client.pem
            Environment=ETCD_KEY_FILE=/etc/kubernetes/ssl/master-client-key.pem
            # Peer Env Vars
            Environment=ETCD_PEER_TRUSTED_CA_FILE=/etc/kubernetes/ssl/ca.pem
            Environment=ETCD_PEER_CERT_FILE=/etc/kubernetes/ssl/master-peer.pem
            Environment=ETCD_PEER_KEY_FILE=/etc/kubernetes/ssl/master-peer-key.pem
    - name: fleet.service
      command: start
    - name: swap.service
      command: start
      content: |
        [Unit]
        Description=Turn on swap

        [Service]
        Type=oneshot
        Environment="SWAPFILE=/swap"
        RemainAfterExit=true
        ExecStartPre=/usr/sbin/losetup -f $SWAPFILE
        ExecStart=/usr/bin/sh -c "/sbin/swapon $(/usr/sbin/losetup -j $SWAPFILE | /usr/bin/cut -d : -f 1)"
        ExecStop=/usr/bin/sh -c "/sbin/swapoff $(/usr/sbin/losetup -j $SWAPFILE | /usr/bin/cut -d : -f 1)"
        ExecStopPost=/usr/bin/sh -c "/usr/sbin/losetup -d $(/usr/sbin/losetup -j $SWAPFILE | /usr/bin/cut -d : -f 1)"

        [Install]
        WantedBy=local.target
""".format(discovery_url, '172.31.29.111'))
