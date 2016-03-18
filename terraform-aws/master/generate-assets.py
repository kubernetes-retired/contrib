#!/usr/bin/env python
import os.path
import subprocess
import argparse


def _write_asset(filename, content):
    with open(os.path.join('assets', filename), 'wt') as f:
        f.write(content)


os.chdir(os.path.abspath(os.path.dirname(__file__)))

cl_parser = argparse.ArgumentParser()
cl_parser.add_argument('dns_address', help='Specify DNS address')
cl_parser.add_argument('region', help='Specify AWS region')
cl_parser.add_argument('public_ip', help='Specify public IP')
cl_parser.add_argument('private_ip', help='Specify private IP')
args = cl_parser.parse_args()

subprocess.check_call(
    ['./generate-certs.py', args.dns_address, args.region, args.public_ip,
     args.private_ip, ]
)

_write_asset('options.env', """FLANNELD_IFACE={0}
FLANNELD_ETCD_ENDPOINTS=https://{0}:2379
FLANNELD_ETCD_CAFILE=/etc/ssl/etcd/ca.pem
FLANNELD_ETCD_CERTFILE=/etc/ssl/etcd/master-client.pem
FLANNELD_ETCD_KEYFILE=/etc/ssl/etcd/master-client-key.pem
""".format(args.private_ip))
_write_asset('40-ExecStartPre-symlink.conf', """[Service]
ExecStartPre=/usr/bin/ln -sf /etc/flannel/options.env /run/flannel/options.env
""")
_write_asset('40-flannel.conf', """[Unit]
Requires=flanneld.service
After=flanneld.service
""")
_write_asset('kubelet.service', """[Service]
Environment=KUBELET_VERSION=v1.1.8_coreos.0
ExecStart=/opt/bin/kubelet-wrapper \\
--cloud-provider=aws \\
--api-servers=https://127.0.0.1:443 \\
--register-node=false \\
--allow-privileged=true \\
--kubeconfig=/etc/kubernetes/kube.conf \\
--config=/etc/kubernetes/manifests \\
--tls-cert-file=/etc/kubernetes/ssl/master-client.pem \\
--tls-private-key-file=/etc/kubernetes/ssl/master-client-key.pem \\
--cluster-dns=10.3.0.10 \\
--cluster-domain=cluster.local \\
--hostname-override={0}
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
""".format(args.private_ip))
_write_asset('kube-apiserver.service', """[Service]
ExecStart=/usr/bin/docker run \\
-p 443:443 \\
-v /etc/kubernetes:/etc/kubernetes:ro \\
-v /etc/kubernetes/ssl:/etc/ssl/etcd:ro \\
gcr.io/google_containers/hyperkube:v1.1.2 \\
/hyperkube apiserver \\
--cloud-provider=aws \\
--bind-address=0.0.0.0 \\
--insecure-bind-address=127.0.0.1 \\
--etcd-config=/etc/kubernetes/etcd.client.conf \\
--allow-privileged=true \\
--service-cluster-ip-range=10.3.0.0/24 \\
--secure-port=443 \\
--advertise-address={0} \\
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
""".format(args.private_ip))
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
_write_asset('kube-podmaster.yaml', """apiVersion: v1
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
    - --whoami={0}
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
""".format(args.private_ip))
_write_asset('kube-controller-manager.yaml', """apiVersion: v1
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
    - --cloud-provider=aws
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
_write_asset('kube-scheduler.yaml', """apiVersion: v1
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
_write_asset('etcd.client.conf', """{{
  "cluster": {{
    "machines": [ "https://{}:2379" ]
  }},
  "config": {{
    "certFile": "/etc/ssl/etcd/master-client.pem",
    "keyFile": "/etc/ssl/etcd/master-client-key.pem"
   }}
}}
""".format(args.private_ip))
_write_asset('kube.conf', """apiVersion: v1
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
""")
_write_asset('dns-addon.yaml', """apiVersion: v1
kind: ReplicationController
metadata:
  name: kube-dns-v11
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    version: v11
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 1
  selector:
    k8s-app: kube-dns
    version: v11
  template:
    metadata:
      labels:
        k8s-app: kube-dns
        version: v11
        kubernetes.io/cluster-service: "true"
    spec:
      containers:
      - name: etcd
        image: gcr.io/google_containers/etcd-amd64:2.2.1
        resources:
          # TODO: Set memory limits when we've profiled the container for large
          # clusters, then set request = limit to keep this container in
          # guaranteed class. Currently, this container falls into the
          # "burstable" category so the kubelet doesn't backoff from restarting
          # it.
          limits:
            cpu: 100m
            memory: 500Mi
          requests:
            cpu: 100m
            memory: 50Mi
        command:
        - /usr/local/bin/etcd
        - -data-dir
        - /var/etcd/data
        - -listen-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -advertise-client-urls
        - http://127.0.0.1:2379,http://127.0.0.1:4001
        - -initial-cluster-token
        - skydns-etcd
        volumeMounts:
        - name: etcd-storage
          mountPath: /var/etcd/data
      - name: kube2sky
        image: gcr.io/google_containers/kube2sky:1.14
        resources:
          # TODO: Set memory limits when we've profiled the container for large
          # clusters, then set request = limit to keep this container in
          # guaranteed class. Currently, this container falls into the
          # "burstable" category so the kubelet doesn't backoff from restarting
          # it.
          limits:
            cpu: 100m
            # Kube2sky watches all pods.
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 50Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
        volumeMounts:
        - name: kubernetes-etc
          mountPath: /etc/kubernetes
          readOnly: true
        - name: etcd-ssl
          mountPath: /etc/ssl/etcd
          readOnly: true
        readinessProbe:
          httpGet:
            path: /readiness
            port: 8081
            scheme: HTTP
          # we poll on pod startup for the Kubernetes master service and
          # only setup the /readiness HTTP server once that's available.
          initialDelaySeconds: 30
          timeoutSeconds: 5
        args:
        # command = "/kube2sky"
        - --domain=cluster.local.
        - --kubecfg-file=/etc/kubernetes/kube.conf
      - name: skydns
        image: gcr.io/google_containers/skydns:2015-10-13-8c72f8c
        resources:
          # TODO: Set memory limits when we've profiled the container for large
          # clusters, then set request = limit to keep this container in
          # guaranteed class. Currently, this container falls into the
          # "burstable" category so the kubelet doesn't backoff from restarting
          # it.
          limits:
            cpu: 100m
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 50Mi
        args:
        # command = "/skydns"
        - -machines=http://127.0.0.1:4001
        - -addr=0.0.0.0:53
        - -ns-rotate=false
        - -domain=cluster.local
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
      - name: healthz
        image: gcr.io/google_containers/exechealthz:1.0
        resources:
          # keep request = limit to keep this container in guaranteed class
          limits:
            cpu: 10m
            memory: 20Mi
          requests:
            cpu: 10m
            memory: 20Mi
        args:
        - -cmd=nslookup kubernetes.default.svc.cluster.local \
127.0.0.1 >/dev/null
        - -port=8080
        ports:
        - containerPort: 8080
          protocol: TCP
      volumes:
      - name: etcd-storage
        emptyDir: {}
      - name: kubernetes-etc
        hostPath:
          path: /etc/kubernetes
      - name: etcd-ssl
        hostPath:
          path: /etc/kubernetes/ssl
      dnsPolicy: Default  # Don't use cluster DNS.
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "KubeDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: 10.3.0.10
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
""")
# _write_asset('nginx-secret.yaml', """apiVersion: "v1"
# kind: "Secret"
# metadata:
#   name: "ssl-proxy-secret"
#   namespace: "default"
# data:
#   proxycert: "{}"
#   proxykey: "{}"
#   dhparam: "{}"
# """.format(b64_cert, b64_key, b64_dhparam))
