#!/bin/sh
sudo chown root:root /etc/kubernetes/ssl/*.pem && \
sudo chown root:root /etc/ssl/etcd/*.pem && \
sudo chmod +x /opt/bin/kubelet-wrapper && \
mkdir -p /home/core/.kube && \
ln -s /etc/kubernetes/kube.conf /home/core/.kube/config && \
sudo systemctl daemon-reload && \
sudo systemctl enable kubelet && \
sudo systemctl start kubelet
