#!/usr/bin/env python
import os.path
import subprocess
import argparse


def _write_asset(filename, content):
    dirname = os.path.join('assets', str(args.node_num))
    if not os.path.exists(dirname):
        os.makedirs(dirname)

    with open(os.path.join(dirname, filename), 'wt') as f:
        f.write(content)


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('node_num', type=int, help='Specify node number')
cl_parser.add_argument('private_ip', help='Specify private IP')
cl_parser.add_argument(
    'master_private_ip', nargs='+', help='Specify master private IP(s)')
args = cl_parser.parse_args()

subprocess.check_call(
    ['./generate-certs.py', str(args.node_num), args.private_ip])

subprocess.check_call([
    './make-cloud-config.py',
    str(args.node_num),
    args.master_private_ip[0]
])

etcd_endpoints = ['https://{0}:2379'.format(x) for x in args.master_private_ip]
_write_asset('options.env', """FLANNELD_IFACE={0}
FLANNELD_ETCD_ENDPOINTS={1}
FLANNELD_ETCD_CAFILE=/etc/ssl/etcd/ca.pem
FLANNELD_ETCD_CERTFILE=/etc/ssl/etcd/worker.pem
FLANNELD_ETCD_KEYFILE=/etc/ssl/etcd/worker-key.pem
""".format(args.private_ip, ','.join(etcd_endpoints)))
_write_asset('40-ExecStartPre-symlink.conf', """[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
""")
_write_asset('40-flannel.conf', """[Unit]
Requires=flanneld.service
After=flanneld.service
""")
api_servers = ['https://{0}:443'.format(x) for x in args.master_private_ip]
_write_asset('kubelet.service', """[Service]
Environment=KUBELET_VERSION=v1.1.8_coreos.0
ExecStart=/opt/bin/kubelet-wrapper \\
--cloud-provider=aws \\
--api_servers={0} \\
--register-node=true \\
--allow-privileged=true \\
--config=/etc/kubernetes/manifests \\
--cluster-dns=10.3.0.10 \\
--cluster-domain=cluster.local \\
--kubeconfig=/home/core/.kube/config \\
--tls-cert-file=/etc/kubernetes/ssl/worker.pem \\
--tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem \\
--hostname-override={1}
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
""".format(','.join(api_servers), args.private_ip))
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
    - --kubeconfig=/home/core/.kube/config
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
