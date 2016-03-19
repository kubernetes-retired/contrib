#!/usr/bin/env python3
import subprocess
import urllib.request
import os.path


os.chdir(os.path.abspath(os.path.dirname(__file__)))


with urllib.request.urlopen('https://discovery.etcd.io/new?size=1') \
        as response:
    discovery_url = response.read().decode()

with open('./assets/kube.conf', 'rt') as f:
    kube_conf = f.read()

with open('./assets/cloud-config', 'wt') as fcloud_config:
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
""".format(kube_conf)
