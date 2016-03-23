#!/usr/bin/env python3
import subprocess
import urllib.request
import os.path
import argparse


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('node_num', help='Specify node number')
cl_parser.add_argument(
    'master_private_ip', help='Specify private IP of masters')
args = cl_parser.parse_args()


with urllib.request.urlopen('https://discovery.etcd.io/new?size=1') \
        as response:
    discovery_url = response.read().decode()

kube_conf = """apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    certificate-authority: /etc/kubernetes/ssl/ca.pem
    server: https://{}:443
users:
- name: kubelet
  user:
    client-certificate: /etc/kubernetes/ssl/worker.pem
    client-key: /etc/kubernetes/ssl/worker-key.pem
contexts:
- context:
    cluster: local
    user: kubelet
""".format(args.master_private_ip[0])
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
  units:
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
""".format(kube_conf))
