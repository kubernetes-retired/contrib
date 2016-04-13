#!/usr/bin/env python3
import subprocess
import urllib.request
from urllib.error import URLError
import os.path
import argparse
import time


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument(
    'master_public_ip', help='Specify public IP of master')
args = cl_parser.parse_args()

attempt = 0
while True:
    attempt += 1
    try:
        with urllib.request.urlopen('https://discovery.etcd.io/new?size=1') \
                as response:
            discovery_url = response.read().decode()
    except URLError:
        if attempt == 5:
            raise
        else:
            time.sleep(attempt)
    else:
        break

with open(os.path.expanduser('~/.ssh/id_rsa.pub'), 'rt') as f:
    ssh_key = f.read().strip()

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
""".format(args.master_public_ip)
kube_conf = '\n'.join([' ' * 6 + l for l in kube_conf.splitlines()])

with open(os.path.join('assets', 'cloud-config'), 'wt') as \
        fcloud_config:
    fcloud_config.write("""#cloud-config

write_files:
  - path: "/etc/kubernetes/kube.conf"
    permissions: "0644"
    owner: "root"
    content: |
{0}
ssh_authorized_keys:
  - "{1}"
coreos:
  update:
    reboot-strategy: "etcd-lock"
  units:
    - name: swap.service
      command: start
      content: |
        [Unit]
        Description=Turn on swap
        After=bootstrap.service
        Requires=bootstrap.service

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
    - name: bootstrap.service
      command: start
      content: |
        [Unit]
        Description=Bootstrap instance
        After=network-online.target
        Requires=network-online.target

        [Service]
        Type=oneshot
        RemainAfterExit=true
        ExecStart=/usr/bin/mkdir -p /tmp/kubernetes-staging
        ExecStart=wget -O /tmp/kubernetes-staging/staging.tar.gz https://storage.googleapis.com/experimentalberlin/staging.tar.gz
        ExecStart=tar xf /tmp/kubernetes-staging/staging.tar.gz -C /tmp/kubernetes-staging/
        ExecStart=/bin/bash /tmp/kubernetes-staging/worker/bootstrap.sh

        [Install]
        WantedBy=local.target
""".format(kube_conf, ssh_key,))
