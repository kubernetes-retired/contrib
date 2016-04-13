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


def _write_addon(filename, parent, content):
    dirname = os.path.join('addons', parent)
    if not os.path.exists(dirname):
        os.makedirs(dirname)

    with open(os.path.join(dirname, filename), 'wt') as f:
        f.write(content)


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('dns_address', help='Specify DNS address')
cl_parser.add_argument('region', help='Specify GCE region')
cl_parser.add_argument('discovery_url', help='Specify etcd discovery URL')
cl_parser.add_argument('public_ip', help='Specify public IP')
args = cl_parser.parse_args()

subprocess.check_call([
    './generate-certs.py', args.dns_address, args.region,
    args.public_ip,
])

subprocess.check_call([
    './make-cloud-config.py',
    args.discovery_url,
])

write_instance_env(is_master=True)

# TODO: Allow for several masters
etcd_endpoints = ['https://staging-master1:2379']
etcd_endpoints_str = ','.join(etcd_endpoints)
write_asset('kubelet.service', """[Service]
Environment=KUBELET_VERSION=v1.1.8_coreos.0
ExecStart=/opt/bin/kubelet-wrapper \\
--cloud-provider=gce \\
--api-servers=https://127.0.0.1:443 \\
--register-node=false \\
--allow-privileged=true \\
--kubeconfig=/etc/kubernetes/kube.conf \\
--config=/etc/kubernetes/manifests \\
--tls-cert-file=/etc/kubernetes/ssl/master-client.pem \\
--tls-private-key-file=/etc/kubernetes/ssl/master-client-key.pem \\
--cluster-dns=10.3.0.10 \\
--cluster-domain=cluster.local \\
--hostname-override=staging-master1
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
""")
write_asset('kube-apiserver.service', """[Service]
ExecStart=/usr/bin/docker run \\
-p 443:443 \\
-v /etc/kubernetes:/etc/kubernetes:ro \\
-v /etc/kubernetes/ssl:/etc/ssl/etcd:ro \\
gcr.io/google_containers/hyperkube:v1.1.2 \\
/hyperkube apiserver \\
--cloud-provider=gce \\
--bind-address=0.0.0.0 \\
--insecure-bind-address=127.0.0.1 \\
--etcd-config=/etc/kubernetes/etcd.client.conf \\
--allow-privileged=true \\
--service-cluster-ip-range=10.3.0.0/24 \\
--secure-port=443 \\
--advertise-address=0.0.0.0 \\
--admission-control=NamespaceLifecycle,NamespaceExists,LimitRanger,\
SecurityContextDeny,ServiceAccount,ResourceQuota \\
--kubelet-certificate-authority=/etc/ssl/etcd/ca.pem \\
--kubelet-client-certificate=/etc/ssl/etcd/master-client.pem \\
--kubelet-client-key=/etc/ssl/etcd/master-client-key.pem \\
--client-ca-file=/etc/ssl/etcd/ca.pem \\
--tls-cert-file=/etc/ssl/etcd/master-client.pem \\
--tls-private-key-file=/etc/ssl/etcd/master-client-key.pem
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
""")
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
    - --master=https://127.0.0.1:443
    - --proxy-mode=iptables
    - --kubeconfig=/etc/kubernetes/kube.conf
    - --v=2
    securityContext:
      privileged: true
    volumeMounts:
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
    - mountPath: /etc/kubernetes
      name: kubernetes
      readOnly: true
    - mountPath: /etc/ssl/etcd
      name: kubernetes-certs
      readOnly: true
  volumes:
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
  - hostPath:
      path: /etc/kubernetes
    name: kubernetes
  - hostPath:
      path: /etc/kubernetes/ssl
    name: kubernetes-certs
""")
write_asset('kube-podmaster.yaml', """apiVersion: v1
kind: Pod
metadata:
  name: podmaster
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: scheduler-elector
    image: quay.io/saltosystems/podmaster:1.1
    command:
    - /podmaster
    - --etcd-config=/etc/kubernetes/etcd.client.conf
    - --key=scheduler
    - --whoami={0}
    - --source-file=/src/manifests/kube-scheduler.yaml
    - --dest-file=/dst/manifests/kube-scheduler.yaml
    volumeMounts:
    - mountPath: /src/manifests
      name: manifest-src
      readOnly: true
    - mountPath: /dst/manifests
      name: manifest-dst
    - mountPath: /etc/kubernetes
      name: kubernetes
      readOnly: true
    - mountPath: /etc/ssl/etcd
      name: secrets
      readOnly: true
  - name: controller-manager-elector
    image: quay.io/saltosystems/podmaster:1.1
    command:
    - /podmaster
    - --etcd-config=/etc/kubernetes/etcd.client.conf
    - --key=controller
    - --whoami=staging-master1
    - --source-file=/src/manifests/kube-controller-manager.yaml
    - --dest-file=/dst/manifests/kube-controller-manager.yaml
    terminationMessagePath: /dev/termination-log
    volumeMounts:
    - mountPath: /src/manifests
      name: manifest-src
      readOnly: true
    - mountPath: /dst/manifests
      name: manifest-dst
    - mountPath: /etc/kubernetes
      name: kubernetes
      readOnly: true
    - mountPath: /etc/ssl/etcd
      name: secrets
      readOnly: true
  volumes:
  - hostPath:
      path: /srv/kubernetes/manifests
    name: manifest-src
  - hostPath:
      path: /etc/kubernetes/manifests
    name: manifest-dst
  - hostPath:
      path: /etc/kubernetes
    name: kubernetes
  - hostPath:
      path: /etc/kubernetes/ssl
    name: secrets
""")
write_asset('kube-controller-manager.yaml', """apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-controller-manager
    image: gcr.io/google_containers/hyperkube:v1.1.2
    command:
    - /hyperkube
    - controller-manager
    - --cloud-provider=gce
    - --service-account-private-key-file=/etc/kubernetes/ssl/\
master-client-key.pem
    - --kubeconfig=/etc/kubernetes/kube.conf
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10252
      initialDelaySeconds: 15
      timeoutSeconds: 1
    volumeMounts:
    - mountPath: /etc/kubernetes
      name: kubernetes
      readOnly: true
    - mountPath: /etc/ssl/etcd
      name: etcd-certs
      readOnly: true
    - mountPath: /etc/kubernetes/ssl
      name: ssl-certs-kubernetes
      readOnly: true
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/kubernetes
    name: kubernetes
  - hostPath:
      path: /etc/ssl/etcd
    name: etcd-certs
  - hostPath:
      path: /etc/kubernetes/ssl
    name: ssl-certs-kubernetes
  - hostPath:
      path: /usr/share/ca-certificates
    name: ssl-certs-host
""")
write_asset('kube-scheduler.yaml', """apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  hostNetwork: true
  containers:
  - name: kube-scheduler
    image: gcr.io/google_containers/hyperkube:v1.1.2
    command:
    - /hyperkube
    - scheduler
    - --kubeconfig=/etc/kubernetes/kube.conf
    livenessProbe:
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10251
      initialDelaySeconds: 15
      timeoutSeconds: 1
    volumeMounts:
    - mountPath: /etc/kubernetes
      name: kubernetes
      readOnly: true
    - mountPath: /etc/ssl/etcd
      name: etcd-certs
      readOnly: true
  volumes:
  - hostPath:
      path: /etc/kubernetes
    name: kubernetes
  - hostPath:
      path: /etc/kubernetes/ssl
    name: etcd-certs
""")
write_asset('etcd.client.conf', """{{
  "cluster": {{
    "machines": [ {0} ]
  }},
  "config": {{
    "certFile": "/etc/ssl/etcd/master-client.pem",
    "keyFile": "/etc/ssl/etcd/master-client-key.pem"
   }}
}}
""".format(','.join(['"{}"'.format(x) for x in etcd_endpoints]),))
# num_heapster_nodes = 1
# metrics_memory = '{}Mi'.format(200 + num_heapster_nodes * 4)
# eventer_memory = '{}Ki'.format(200 * 1024 + num_heapster_nodes * 500)
metrics_memory = '200Mi'
eventer_memory = '200Mi'
_write_addon('heapster-controller.yaml', 'cluster-monitoring', """
apiVersion: v1
kind: ReplicationController
metadata:
  name: heapster-v1.0.0
  namespace: kube-system
  labels:
    k8s-app: heapster
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: heapster
  template:
    metadata:
      labels:
        k8s-app: heapster
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
        - image: gcr.io/google_containers/heapster:v1.0.0
          name: heapster
          resources:
            # keep request = limit to keep this container in guaranteed class
            limits:
              cpu: 100m
              memory: {0}
            requests:
              cpu: 100m
              memory: {0}
          command:
            - /heapster
            - --source=kubernetes.summary_api:''
            - --sink=influxdb:http://monitoring-influxdb:8086
            - --metric_resolution=60s
        - image: gcr.io/google_containers/heapster:v1.0.0
          name: eventer
          resources:
            # keep request = limit to keep this container in guaranteed class
            limits:
              cpu: 100m
              memory: {1}
            requests:
              cpu: 100m
              memory: {1}
          command:
            - /eventer
            - --source=kubernetes:''
            - --sink=influxdb:http://monitoring-influxdb:8086
""".format(metrics_memory, eventer_memory))
# write_asset('nginx-secret.yaml', """apiVersion: "v1"
# kind: "Secret"
# metadata:
#   name: "ssl-proxy-secret"
#   namespace: "default"
# data:
#   proxycert: "{}"
#   proxykey: "{}"
#   dhparam: "{}"
# """.format(b64_cert, b64_key, b64_dhparam))
