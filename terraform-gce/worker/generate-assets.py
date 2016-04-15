#!/usr/bin/env python3
import os.path
import subprocess
import argparse
import sys
import functools

root_dir = os.path.abspath(
    os.path.normpath(os.path.join(os.path.dirname(__file__), '..')))
sys.path.insert(0, root_dir)

import common
from common import write_instance_env, write_asset


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument(
    'master_public_ip', nargs='+', help='Specify master public IP(s)')
args = cl_parser.parse_args()

subprocess.check_call(
    ['./generate-certs.py', ])

subprocess.check_call([
    './make-cloud-config.py',
    args.master_public_ip[0],
])

# TODO: Allow for more masters
etcd_endpoints = ['https://{0}:2379'.format(args.master_public_ip)]
api_servers = ['https://{0}:443'.format(x) for x in args.master_public_ip]
write_asset('kubelet.service', """[Service]
Environment=KUBELET_VERSION=v1.2.0_coreos.1
ExecStart=/opt/bin/kubelet-wrapper \\
--cloud-provider=gce \\
--api_servers={0} \\
--register-node=true \\
--allow-privileged=true \\
--config=/etc/kubernetes/manifests \\
--cluster-dns=10.3.0.10 \\
--cluster-domain=cluster.local \\
--kubeconfig=/etc/kubernetes/kube.conf \\
--tls-cert-file=/etc/kubernetes/ssl/worker.pem \\
--tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem
#--hostname-override=
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
""".format(','.join(api_servers),))
write_asset('kube-proxy.yaml', """apiVersion: v1
kind: Pod
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-proxy
    image: gcr.io/google_containers/hyperkube:v1.1.2
    command:
    - /hyperkube
    - proxy
    - --kubeconfig=/etc/kubernetes/kube.conf
    - --proxy-mode=iptables
    - --v=2
    securityContext:
      privileged: true
    volumeMounts:
      - mountPath: /etc/ssl/certs
        name: "ssl-certs"
      - mountPath: /etc/kubernetes
        name: "kubernetes-etc"
        readOnly: true
      - mountPath: /etc/ssl/etcd
        name: "etc-kube-ssl"
        readOnly: true
  volumes:
    - name: "ssl-certs"
      hostPath:
        path: "/usr/share/ca-certificates"
    - name: "kubernetes-etc"
      hostPath:
        path: "/etc/kubernetes"
    - name: "etc-kube-ssl"
      hostPath:
        path: "/etc/ssl/etcd"
""")
