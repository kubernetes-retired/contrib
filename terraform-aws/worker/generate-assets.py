#!/usr/bin/env python
import os.path
import subprocess
import argparse


def _write_asset(filename, content):
    with open(os.path.join('assets', filename), 'wt') as f:
        f.write(content)


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('private_ip', help='Specify private IP')
cl_parser.add_argument('master_private_ip', help='Specify master private IP')
args = cl_parser.parse_args()

subprocess.check_call(['./generate-certs.py', args.private_ip])

_write_asset('options.env', """FLANNELD_IFACE={}
FLANNELD_ETCD_ENDPOINTS=https://{}:2379
FLANNELD_ETCD_CAFILE=/etc/ssl/etcd/ca.pem
FLANNELD_ETCD_CERTFILE=/etc/ssl/etcd/worker.pem
FLANNELD_ETCD_KEYFILE=/etc/ssl/etcd/worker-key.pem
""".format(args.private_ip, args.master_private_ip))
_write_asset('40-ExecStartPre-symlink.conf', """[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
""")
_write_asset('40-flannel.conf', """[Unit]
Requires=flanneld.service
After=flanneld.service
""")
_write_asset('kubelet.service', """[Service]
Environment=KUBELET_VERSION=v1.1.8_coreos.0
ExecStart=/usr/lib/coreos/kubelet-wrapper \\
--cloud-provider=aws \\
--api_servers=https://{0}:443 \\
--register-node=true \\
--allow-privileged=true \\
--config=/etc/kubernetes/manifests \\
--cluster-dns=10.3.0.10 \\
--cluster-domain=cluster.local \\
--kubeconfig=/etc/kubernetes/kube.conf \\
--tls-cert-file=/etc/kubernetes/ssl/worker.pem \\
--tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem \\
--hostname-override={1}
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
""".format(args.master_private_ip, args.private_ip))
_write_asset('kube-proxy.yaml', """apiVersion: v1
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
      - mountPath: /etc/kubernetes/kube.conf
        name: "kubeconfig"
        readOnly: true
      - mountPath: /etc/ssl/etcd
        name: "etc-kube-ssl"
        readOnly: true
  volumes:
    - name: "ssl-certs"
      hostPath:
        path: "/usr/share/ca-certificates"
    - name: "kubeconfig"
      hostPath:
        path: "/etc/kubernetes/kube.conf"
    - name: "etc-kube-ssl"
      hostPath:
        path: "/etc/ssl/etcd"
""")
_write_asset('kube.conf', """apiVersion: v1
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
""".format(args.master_private_ip))
