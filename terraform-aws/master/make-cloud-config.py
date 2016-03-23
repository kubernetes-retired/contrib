#!/usr/bin/env python3
import subprocess
import os.path
import argparse


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('node_num', help='Specify node number')
cl_parser.add_argument('discovery_url', help='Specify etcd discovery URL')
cl_parser.add_argument(
    'private_ip', nargs='+', help='Specify private IPs of masters')
args = cl_parser.parse_args()

kube_conf = """apiVersion: v1
kind: Config
clusters:
- name: kube
  cluster:
    server: https://127.0.0.1:443
    certificate-authority: /etc/kubernetes/ssl/ca.pem
users:
- name: kubelet
  user:
    client-certificate: /etc/kubernetes/ssl/master-client.pem
    client-key: /etc/kubernetes/ssl/master-client-key.pem
contexts:
- context:
    cluster: kube
    user: kubelet
"""
kube_conf = '\n'.join([' ' * 6 + l for l in kube_conf.splitlines()])

with open(os.path.join('assets', args.node_num, 'cloud-config'), 'wt') as \
        fcloud_config:
    fcloud_config.write("""#cloud-config
write_files:
  - path: "/home/core/.kube/config"
    permissions: "0600"
    owner: "core"
    content: |
{0}
coreos:
  etcd2:
    discovery: {1}
    initial-advertise-peer-urls: https://{2}:2380
    listen-peer-urls: https://{2}:2380
    listen-client-urls: https://0.0.0.0:2379
    advertise-client-urls: https://{2}:2379
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
""".format(kube_conf, args.discovery_url, args.private_ip[0],))
