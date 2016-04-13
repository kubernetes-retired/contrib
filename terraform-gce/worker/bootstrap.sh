#!/bin/sh
make_dir () {
  sudo mkdir -p $1 && \
  sudo chown root:admin $1 && \
  sudo chmod 0775  $1
}

cd /tmp/kubernetes-staging

sudo groupadd admin && \
sudo usermod core -a -G admin && \
sudo usermod root -a -G admin && \
make_dir /opt/bin && \
make_dir /etc/kubernetes && \
make_dir /etc/kubernetes/ssl && \
make_dir /etc/kubernetes/manifests && \
make_dir /etc/ssl/etcd && \
make_dir /etc/flannel && \
make_dir /etc/systemd/system && \
make_dir /etc/systemd/system/flanneld.service.d && \
make_dir /etc/systemd/system/docker.service.d && \
sudo fallocate -l 2048m /swap && \
sudo chmod 600 /swap && \
sudo chattr +C /swap && \
sudo mkswap /swap

sudo cp worker/assets/kubelet.service /etc/systemd/system/kubelet.service
sudo cp worker/assets/kube-proxy.yaml /etc/kubernetes/manifests/kube-proxy.yaml
sudo cp common/assets/kubelet-wrapper /opt/bin/kubelet-wrapper
sudo cp worker/assets/certificates/ca.pem /etc/ssl/etcd/ca.pem
sudo cp worker/assets/certificates/worker-client.pem /etc/ssl/etcd/worker.pem
sudo cp worker/assets/certificates/worker-client-key.pem /etc/ssl/etcd/worker-key.pem
sudo cp worker/assets/certificates/ca.pem /etc/kubernetes/ssl/ca.pem
sudo cp worker/assets/certificates/worker-client.pem /etc/kubernetes/ssl/worker.pem
sudo cp worker/assets/certificates/worker-client-key.pem /etc/kubernetes/ssl/worker-key.pem

sudo chown root:root /etc/kubernetes/ssl/*.pem && \
sudo chown root:root /etc/ssl/etcd/*.pem && \
sudo chmod +x /opt/bin/kubelet-wrapper && \
mkdir -p /home/core/.kube && \
ln -s /etc/kubernetes/kube.conf /home/core/.kube/config && \
sudo systemctl daemon-reload && \
sudo systemctl enable kubelet && \
sudo systemctl start kubelet
