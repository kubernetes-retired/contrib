#! /bin/bash
mkdir -p /var/cache/kubernetes-install
cd /var/cache/kubernetes-install
readonly SALT_MASTER='172.20.0.9'
readonly INSTANCE_PREFIX='${CLUSTER_ID}'
readonly NODE_INSTANCE_PREFIX='${CLUSTER_ID}-minion'
readonly CLUSTER_IP_RANGE='${CLUSTER_IP_RANGE}'
readonly ALLOCATE_NODE_CIDRS='true'
readonly SERVER_BINARY_TAR_URL='https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/kubernetes-server-linux-amd64.tar.gz'
readonly SALT_TAR_URL='https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/kubernetes-salt.tar.gz'
readonly ZONE='${aws_availability_zone}'
readonly KUBE_USER='${KUBE_USER}'
readonly KUBE_PASSWORD='${KUBE_PASSWORD}'
readonly SERVICE_CLUSTER_IP_RANGE='10.0.0.0/16'
readonly ENABLE_CLUSTER_MONITORING='${ENABLE_CLUSTER_MONITORING}'
readonly ENABLE_CLUSTER_LOGGING='${ENABLE_CLUSTER_LOGGING}'
readonly ENABLE_NODE_LOGGING='${ENABLE_NODE_LOGGING}'
readonly LOGGING_DESTINATION='${LOGGING_DESTINATION}'
readonly ELASTICSEARCH_LOGGING_REPLICAS='${ELASTICSEARCH_LOGGING_REPLICAS}'
readonly ENABLE_CLUSTER_DNS='${ENABLE_CLUSTER_DNS}'
readonly ENABLE_CLUSTER_UI='${ENABLE_CLUSTER_UI}'
readonly DNS_REPLICAS='${DNS_REPLICAS}'
readonly DNS_SERVER_IP='${DNS_SERVER_IP}'
readonly DNS_DOMAIN='${DNS_DOMAIN}'
readonly ADMISSION_CONTROL='${ADMISSION_CONTROL}'
readonly MASTER_IP_RANGE='${MASTER_IP_RANGE}'
readonly KUBELET_TOKEN='${KUBELET_TOKEN}'
readonly KUBE_PROXY_TOKEN='${KUBE_PROXY_TOKEN}'
readonly DOCKER_STORAGE='${DOCKER_STORAGE}'
readonly DOCKER_OPTS='${EXTRA_DOCKER_OPTS}'
readonly NUM_MINIONS='${NUM_MINIONS}'
readonly MASTER_EXTRA_SANS='IP:10.0.0.1,DNS:kubernetes,DNS:kubernetes.default,DNS:kubernetes.default.svc,DNS:kubernetes.default.svc.cluster.local,DNS:kubernetes-master'



apt-get update
apt-get install --yes curl

download-or-bust() {
  local -r url="$1"
  local -r file="$${url##*/}"
  rm -f "$file"
  until [[ -e "$${1##*/}" ]]; do
    echo "Downloading file ($1)"
    curl --ipv4 -Lo "$file" --connect-timeout 20 --retry 6 --retry-delay 10 "$1"
    md5sum "$file"
  done
}



install-salt() {
  local salt_mode="$1"

  if dpkg -s salt-minion &>/dev/null; then
    echo "== SaltStack already installed, skipping install step =="
    return
  fi

  echo "== Refreshing package database =="
  until apt-get update; do
    echo "== apt-get update failed, retrying =="
    echo sleep 5
  done

  mkdir -p /var/cache/salt-install
  cd /var/cache/salt-install

  DEBS=(
    libzmq3_3.2.3+dfsg-1~bpo70~dst+1_amd64.deb
    python-zmq_13.1.0-1~bpo70~dst+1_amd64.deb
    salt-common_2014.1.13+ds-1~bpo70+1_all.deb
  )
  if [[ "$${salt_mode}" == "master" ]]; then
    DEBS+=( salt-master_2014.1.13+ds-1~bpo70+1_all.deb )
  fi
  DEBS+=( salt-minion_2014.1.13+ds-1~bpo70+1_all.deb )
  URL_BASE="https://storage.googleapis.com/kubernetes-release/salt"

  for deb in "$${DEBS[@]}"; do
    if [ ! -e "$${deb}" ]; then
      download-or-bust "$${URL_BASE}/$${deb}"
    fi
  done

  for deb in "$${DEBS[@]}"; do
    echo "== Installing $${deb}, ignore dependency complaints (will fix later) =="
    dpkg --skip-same-version --force-depends -i "$${deb}"
  done

  # This will install any of the unmet dependencies from above.
  echo "== Installing unmet dependencies =="
  until apt-get install -f -y; do
    echo "== apt-get install failed, retrying =="
    echo sleep 5
  done

  # Log a timestamp
  echo "== Finished installing Salt =="
}



block_devices=()

ephemeral_devices=$(curl --silent http://169.254.169.254/2014-11-05/meta-data/block-device-mapping/ | grep ephemeral)
for ephemeral_device in $ephemeral_devices; do
  echo "Checking ephemeral device: $${ephemeral_device}"
  aws_device=$(curl --silent http://169.254.169.254/2014-11-05/meta-data/block-device-mapping/$${ephemeral_device})

  device_path=""
  if [ -b /dev/$aws_device ]; then
    device_path="/dev/$aws_device"
  else
    # Check for the xvd-style name
    xvd_style=$(echo $aws_device | sed "s/sd/xvd/")
    if [ -b /dev/$xvd_style ]; then
      device_path="/dev/$xvd_style"
    fi
  fi

  if [[ -z $${device_path} ]]; then
    echo "  Could not find disk: $${ephemeral_device}@$${aws_device}"
  else
    echo "  Detected ephemeral disk: $${ephemeral_device}@$${device_path}"
    block_devices+=($${device_path})
  fi
done

move_docker=""
move_kubelet=""

apt-get update

docker_storage=$${DOCKER_STORAGE:-aufs}

if [[ $${#block_devices[@]} == 0 ]]; then
  echo "No ephemeral block devices found; will use aufs on root"
  docker_storage="aufs"
else
  echo "Block devices: $${block_devices[@]}"

  # Remove any existing mounts
  for block_device in $${block_devices}; do
    echo "Unmounting $${block_device}"
    /bin/umount $${block_device}
    sed -i -e "\|^$${block_device}|d" /etc/fstab
  done

  if [[ $${docker_storage} == "btrfs" ]]; then
    apt-get install --yes btrfs-tools

    if [[ $${#block_devices[@]} == 1 ]]; then
      echo "One ephemeral block device found; formatting with btrfs"
      mkfs.btrfs -f $${block_devices[0]}
    else
      echo "Found multiple ephemeral block devices, formatting with btrfs as RAID-0"
      mkfs.btrfs -f --data raid0 $${block_devices[@]}
    fi
    echo "$${block_devices[0]}  /mnt/ephemeral  btrfs  noatime  0 0" >> /etc/fstab
    mkdir -p /mnt/ephemeral
    mount /mnt/ephemeral

    mkdir -p /mnt/ephemeral/kubernetes

    move_docker="/mnt/ephemeral"
    move_kubelet="/mnt/ephemeral/kubernetes"
  elif [[ $${docker_storage} == "aufs-nolvm" ]]; then
    if [[ $${#block_devices[@]} != 1 ]]; then
      echo "aufs-nolvm selected, but multiple ephemeral devices were found; only the first will be available"
    fi

    mkfs -t ext4 $${block_devices[0]}
    echo "$${block_devices[0]}  /mnt/ephemeral  ext4     noatime  0 0" >> /etc/fstab
    mkdir -p /mnt/ephemeral
    mount /mnt/ephemeral

    mkdir -p /mnt/ephemeral/kubernetes

    move_docker="/mnt/ephemeral"
    move_kubelet="/mnt/ephemeral/kubernetes"
  elif [[ $${docker_storage} == "devicemapper" || $${docker_storage} == "aufs" ]]; then
    # We always use LVM, even with one device
    # In devicemapper mode, Docker can use LVM directly
    # Also, fewer code paths are good
    echo "Using LVM2 and ext4"
    apt-get install --yes lvm2

    # Don't output spurious "File descriptor X leaked on vgcreate invocation."
    # Known bug: e.g. Ubuntu #591823
    export LVM_SUPPRESS_FD_WARNINGS=1

    for block_device in $${block_devices}; do
      pvcreate $${block_device}
    done
    vgcreate vg-ephemeral $${block_devices[@]}

    if [[ $${docker_storage} == "devicemapper" ]]; then
      # devicemapper thin provisioning, managed by docker
      # This is the best option, but it is sadly broken on most distros
      # Bug: https://github.com/docker/docker/issues/4036

      # 80% goes to the docker thin-pool; we want to leave some space for host-volumes
      lvcreate -l 80%VG --thinpool docker-thinpool vg-ephemeral

      DOCKER_OPTS="$${DOCKER_OPTS} --storage-opt dm.thinpooldev=/dev/mapper/vg--ephemeral-docker--thinpool"
      # Note that we don't move docker; docker goes direct to the thinpool

      # Remaining space (20%) is for kubernetes data
      # TODO: Should this be a thin pool?  e.g. would we ever want to snapshot this data?
      lvcreate -l 100%FREE -n kubernetes vg-ephemeral
      mkfs -t ext4 /dev/vg-ephemeral/kubernetes
      mkdir -p /mnt/ephemeral/kubernetes
      echo "/dev/vg-ephemeral/kubernetes  /mnt/ephemeral/kubernetes  ext4  noatime  0 0" >> /etc/fstab
      mount /mnt/ephemeral/kubernetes

      move_kubelet="/mnt/ephemeral/kubernetes"
     else
      # aufs

      # We used to split docker & kubernetes, but we no longer do that, because
      # host volumes go into the kubernetes area, and it is otherwise very easy
      # to fill up small volumes.

      release=`lsb_release -c -s`
      if [[ "$${release}" != "wheezy" ]] ; then
        lvcreate -l 100%FREE --thinpool pool-ephemeral vg-ephemeral

        THINPOOL_SIZE=$(lvs vg-ephemeral/pool-ephemeral -o LV_SIZE --noheadings --units M --nosuffix)
        lvcreate -V$${THINPOOL_SIZE}M -T vg-ephemeral/pool-ephemeral -n ephemeral
      else
        # Thin provisioning not supported by Wheezy
        echo "Detected wheezy; won't use LVM thin provisioning"
        lvcreate -l 100%VG -n ephemeral vg-ephemeral
      fi

      mkfs -t ext4 /dev/vg-ephemeral/ephemeral
      mkdir -p /mnt/ephemeral
      echo "/dev/vg-ephemeral/ephemeral  /mnt/ephemeral  ext4  noatime  0 0" >> /etc/fstab
      mount /mnt/ephemeral

      mkdir -p /mnt/ephemeral/kubernetes

      move_docker="/mnt/ephemeral"
      move_kubelet="/mnt/ephemeral/kubernetes"
     fi
 else
    echo "Ignoring unknown DOCKER_STORAGE: $${docker_storage}"
  fi
fi


if [[ $${docker_storage} == "btrfs" ]]; then
  DOCKER_OPTS="$${DOCKER_OPTS} -s btrfs"
elif [[ $${docker_storage} == "aufs-nolvm" || $${docker_storage} == "aufs" ]]; then
  # Install aufs kernel module
  apt-get install --yes linux-image-extra-$(uname -r)

  # Install aufs tools
  apt-get install --yes aufs-tools

  DOCKER_OPTS="$${DOCKER_OPTS} -s aufs"
elif [[ $${docker_storage} == "devicemapper" ]]; then
  DOCKER_OPTS="$${DOCKER_OPTS} -s devicemapper"
else
  echo "Ignoring unknown DOCKER_STORAGE: $${docker_storage}"
fi

if [[ -n "$${move_docker}" ]]; then
  # Move docker to e.g. /mnt
  if [[ -d /var/lib/docker ]]; then
    mv /var/lib/docker $${move_docker}/
  fi
  mkdir -p $${move_docker}/docker
  ln -s $${move_docker}/docker /var/lib/docker
  DOCKER_ROOT="$${move_docker}/docker"
  DOCKER_OPTS="$${DOCKER_OPTS} -g $${DOCKER_ROOT}"
fi

if [[ -n "$${move_kubelet}" ]]; then
  # Move /var/lib/kubelet to e.g. /mnt
  # (the backing for empty-dir volumes can use a lot of space!)
  if [[ -d /var/lib/kubelet ]]; then
    mv /var/lib/kubelet $${move_kubelet}/
  fi
  mkdir -p $${move_kubelet}/kubelet
  ln -s $${move_kubelet}/kubelet /var/lib/kubelet
  KUBELET_ROOT="$${move_kubelet}/kubelet"
fi




echo "Waiting for master pd to be attached"
attempt=0
while true; do
  echo Attempt "$(($attempt+1))" to check for /dev/xvdb
  if [[ -e /dev/xvdb ]]; then
    echo "Found /dev/xvdb"
    break
  fi
  attempt=$(($attempt+1))
  sleep 1
done

echo "Mounting master-pd"
mkdir -p /mnt/master-pd
mkfs -t ext4 /dev/xvdb
echo "/dev/xvdb  /mnt/master-pd  ext4  noatime  0 0" >> /etc/fstab
mount /mnt/master-pd

mkdir -m 700 -p /mnt/master-pd/var/etcd
mkdir -p /mnt/master-pd/srv/kubernetes
mkdir -p /mnt/master-pd/srv/salt-overlay
mkdir -p /mnt/master-pd/srv/sshproxy

ln -s -f /mnt/master-pd/var/etcd /var/etcd
ln -s -f /mnt/master-pd/srv/kubernetes /srv/kubernetes
ln -s -f /mnt/master-pd/srv/sshproxy /srv/sshproxy
ln -s -f /mnt/master-pd/srv/salt-overlay /srv/salt-overlay

if ! id etcd &>/dev/null; then
  useradd -s /sbin/nologin -d /var/etcd etcd
fi
chown -R etcd /mnt/master-pd/var/etcd
chgrp -R etcd /mnt/master-pd/var/etcd



mkdir -p /srv/salt-overlay/pillar
cat <<EOF >/srv/salt-overlay/pillar/cluster-params.sls
instance_prefix: '$(echo "$INSTANCE_PREFIX" | sed -e "s/'/''/g")'
node_instance_prefix: '$(echo "$NODE_INSTANCE_PREFIX" | sed -e "s/'/''/g")'
cluster_cidr: '$(echo "$CLUSTER_IP_RANGE" | sed -e "s/'/''/g")'
allocate_node_cidrs: '$(echo "$ALLOCATE_NODE_CIDRS" | sed -e "s/'/''/g")'
service_cluster_ip_range: '$(echo "$SERVICE_CLUSTER_IP_RANGE" | sed -e "s/'/''/g")'
enable_cluster_monitoring: '$(echo "$ENABLE_CLUSTER_MONITORING" | sed -e "s/'/''/g")'
enable_cluster_logging: '$(echo "$ENABLE_CLUSTER_LOGGING" | sed -e "s/'/''/g")'
enable_cluster_ui: '$(echo "$ENABLE_CLUSTER_UI" | sed -e "s/'/''/g")'
enable_node_logging: '$(echo "$ENABLE_NODE_LOGGING" | sed -e "s/'/''/g")'
logging_destination: '$(echo "$LOGGING_DESTINATION" | sed -e "s/'/''/g")'
elasticsearch_replicas: '$(echo "$ELASTICSEARCH_LOGGING_REPLICAS" | sed -e "s/'/''/g")'
enable_cluster_dns: '$(echo "$ENABLE_CLUSTER_DNS" | sed -e "s/'/''/g")'
dns_replicas: '$(echo "$DNS_REPLICAS" | sed -e "s/'/''/g")'
dns_server: '$(echo "$DNS_SERVER_IP" | sed -e "s/'/''/g")'
dns_domain: '$(echo "$DNS_DOMAIN" | sed -e "s/'/''/g")'
admission_control: '$(echo "$ADMISSION_CONTROL" | sed -e "s/'/''/g")'
num_nodes: $(echo "$${NUM_MINIONS}")
EOF

readonly BASIC_AUTH_FILE="/srv/salt-overlay/salt/kube-apiserver/basic_auth.csv"
if [ ! -e "$${BASIC_AUTH_FILE}" ]; then
  mkdir -p /srv/salt-overlay/salt/kube-apiserver
  (umask 077;
    echo "$${KUBE_PASSWORD},$${KUBE_USER},admin" > "$${BASIC_AUTH_FILE}")
fi

kubelet_token=$KUBELET_TOKEN
kube_proxy_token=$KUBE_PROXY_TOKEN

mkdir -p /srv/salt-overlay/salt/kube-apiserver
readonly KNOWN_TOKENS_FILE="/srv/salt-overlay/salt/kube-apiserver/known_tokens.csv"
(umask u=rw,go= ; echo "$kubelet_token,kubelet,kubelet" > $KNOWN_TOKENS_FILE ;
echo "$kube_proxy_token,kube_proxy,kube_proxy" >> $KNOWN_TOKENS_FILE)

mkdir -p /srv/salt-overlay/salt/kubelet
kubelet_auth_file="/srv/salt-overlay/salt/kubelet/kubernetes_auth"
(umask u=rw,go= ; echo "{\"BearerToken\": \"$kubelet_token\", \"Insecure\": true }" > $kubelet_auth_file)

mkdir -p /srv/salt-overlay/salt/kube-proxy
kube_proxy_kubeconfig_file="/srv/salt-overlay/salt/kube-proxy/kubeconfig"
cat > "$${kube_proxy_kubeconfig_file}" <<EOF
apiVersion: v1
kind: Config
users:
- name: kube-proxy
  user:
    token: $${kube_proxy_token}
clusters:
- name: local
  cluster:
     insecure-skip-tls-verify: true
contexts:
- context:
    cluster: local
    user: kube-proxy
  name: service-account-context
current-context: service-account-context
EOF

mkdir -p /srv/salt-overlay/salt/kubelet
kubelet_kubeconfig_file="/srv/salt-overlay/salt/kubelet/kubeconfig"
cat > "$${kubelet_kubeconfig_file}" <<EOF
apiVersion: v1
kind: Config
users:
- name: kubelet
  user:
    token: $${kubelet_token}
clusters:
- name: local
  cluster:
     insecure-skip-tls-verify: true
contexts:
- context:
    cluster: local
    user: kubelet
  name: service-account-context
current-context: service-account-context
EOF

service_accounts=("system:scheduler" "system:controller_manager" "system:logging" "system:monitoring" "system:dns")
for account in "$${service_accounts[@]}"; do
  token=$(dd if=/dev/urandom bs=128 count=1 2>/dev/null | base64 | tr -d "=+/" | dd bs=32 count=1 2>/dev/null)
  echo "$${token},$${account},$${account}" >> "$${KNOWN_TOKENS_FILE}"
done




echo "Downloading binary release tar ($SERVER_BINARY_TAR_URL)"
download-or-bust "$SERVER_BINARY_TAR_URL"

echo "Downloading binary release tar ($SALT_TAR_URL)"
download-or-bust "$SALT_TAR_URL"

echo "Unpacking Salt tree"
rm -rf kubernetes
tar xzf "$${SALT_TAR_URL##*/}"

echo "Running release install script"
sudo kubernetes/saltbase/install.sh "$${SERVER_BINARY_TAR_URL##*/}"


mkdir -p /etc/salt/minion.d
echo "master: $SALT_MASTER" > /etc/salt/minion.d/master.conf

cat <<EOF >/etc/salt/minion.d/grains.conf
grains:
  roles:
    - kubernetes-master
  cloud: aws
  cbr-cidr: "$${MASTER_IP_RANGE}"
EOF

if [[ -n "$${DOCKER_OPTS}" ]]; then
  cat <<EOF >>/etc/salt/minion.d/grains.conf
  docker_opts: '$(echo "$DOCKER_OPTS" | sed -e "s/'/''/g")'
EOF
fi

if [[ -n "$${DOCKER_ROOT}" ]]; then
  cat <<EOF >>/etc/salt/minion.d/grains.conf
  docker_root: '$(echo "$DOCKER_ROOT" | sed -e "s/'/''/g")'
EOF
fi

if [[ -n "$${KUBELET_ROOT}" ]]; then
  cat <<EOF >>/etc/salt/minion.d/grains.conf
  kubelet_root: '$(echo "$KUBELET_ROOT" | sed -e "s/'/''/g")'
EOF
fi

if [[ -n "$${MASTER_EXTRA_SANS}" ]]; then
  cat <<EOF >>/etc/salt/minion.d/grains.conf
  master_extra_sans: '$(echo "$MASTER_EXTRA_SANS" | sed -e "s/'/''/g")'
EOF
fi

mkdir -p /etc/salt/master.d
cat <<EOF >/etc/salt/master.d/auto-accept.conf
auto_accept: True
EOF

cat <<EOF >/etc/salt/master.d/reactor.conf
reactor:
  - 'salt/minion/*/start':
    - /srv/reactor/highstate-new.sls
EOF

install-salt master

service salt-master start
service salt-minion start


