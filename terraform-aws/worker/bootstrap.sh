make_dir () {
   sudo mkdir -p $1 && \
   sudo chown root:admin $1 && \
   sudo chmod 0775  $1
}

sudo groupadd admin && \
sudo usermod core -a -G admin && \
sudo usermod root -a -G admin && \
make_dir /etc/kubernetes && \
make_dir /etc/kubernetes/ssl && \
make_dir /etc/kubernetes/manifests && \
make_dir /etc/ssl/etcd && \
make_dir /etc/flannel && \
make_dir /etc/systemd/system && \
make_dir /etc/systemd/system/flanneld.service.d && \
make_dir /etc/systemd/system/docker.service.d
