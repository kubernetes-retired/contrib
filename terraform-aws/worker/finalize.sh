#!/bin/sh
sudo chown root:root /etc/kubernetes/ssl/*.pem && \
sudo chown root:root /etc/ssl/etcd/*.pem && \
sudo systemctl daemon-reload && \
sudo systemctl enable kubelet && \
sudo systemctl start kubelet
