#!/bin/sh
CERTSDIR=/etc/kubernetes/ssl

sudo chown root:root /etc/kubernetes/ssl/*.pem && \
sudo chown root:root /etc/ssl/etcd/*.pem && \
chmod +x /opt/bin/kubectl && \
chmod +x /opt/bin/kubelet-wrapper && \
mkdir -p /home/core/.kube && \
ln -s /etc/kubernetes/kube.conf /home/core/.kube/config && \
sudo systemctl daemon-reload && \
sudo systemctl enable kubelet && \
sudo systemctl start kubelet && \
sudo systemctl enable kube-apiserver && \
sudo systemctl start kube-apiserver
