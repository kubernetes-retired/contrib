#!/bin/bash
# used to install kubectl inside circleci.
# $1 = the url for your kubeconfig file
# 

KUBECONFIGURL=$1

#set -x

PKG_MANAGER=$( command -v yum || command -v apt-get ) || echo "Neither yum nor apt-get found"

#make sure sudo is installed
if [ ! -e "/usr/bin/sudo" ]; then
   ${PKG_MANAGER} install -y sudo
   # remove default setting of requiretty if it exists
   sed -i '/Defaults requiretty/d' /etc/sudoers
fi

#make sure wget is installed
if [ ! -e "/usr/bin/wget" ]; then
   ${PKG_MANAGER} install -y wget
fi

#make sure jq is installed
if [ ! -e "/usr/bin/jq" ]; then
    sudo ${PKG_MANAGER} install -y jq
fi

#make sure envsubst is installed
if [ ! -e "/usr/bin/envsubst" ]; then
    sudo ${PKG_MANAGER} install -y gettext
fi

# make the temp directory
if [ ! -e ~/.kube ]; then
    mkdir -p ~/.kube;
fi

if [ ! -e ~/.kube/kubectl ]; then
    wget https://storage.googleapis.com/kubernetes-release/release/v1.0.6/bin/linux/amd64/kubectl -O ~/.kube/kubectl
    chmod +x ~/.kube/kubectl
fi

if [ ! -e ~/.kube/config ]; then
    wget ${KUBECONFIGURL} -O ~/.kube/config
fi

~/.kube/kubectl version

## uncomment if you need to add an intermediate certificate to get push working
#sudo mkdir -p /etc/docker/certs.d/docker-registry.concur.com
#sudo wget https://s3location/digicert.crt -O /etc/docker/certs.d/docker-registry.concur.com/ca.crt