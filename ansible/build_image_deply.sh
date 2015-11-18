#!/bin/bash -x

set -e

SVC_NAME=$1 # eg. friend-service
VERSION=$2 # eg. 0.1 or 1 or v1

if [[ "$1" == "" ]];  then
  echo "Please provide service name."
  exit 1
fi
if [[ "$2" == "" ]];  then
  echo "Please provide service version."
  exit 2
fi
if [[ ! -f /home/dockerbuild/${SVC_NAME}.tar.gz ]]; then
  echo "Please provide service ${SVC_NAME}.tar.gz file."
  exit 3
fi

SVC_PATH="/home/dockerbuild/${SVC_NAME}"

ansible all -i "localhost," -c local -m file -a "path=/home/dockerbuild/${SVC_NAME} state=directory"
ansible all -i "localhost," -c local -m copy -a "src=/home/dockerbuild/${SVC_NAME}.tar.gz dest=/home/dockerbuild/${SVC_NAME}/${SVC_NAME}.tar.gz"

echo "######################### build image ############################"

# decompress
cd "${SVC_PATH}"
tar xvf ${SVC_NAME}.tar.gz

rm -f "${SVC_NAME}.tar.gz"
cd "${SVC_PATH}/dist/"
docker build -t dockerimages.yinnut.com:15043/${SVC_NAME}:${VERSION} . # use app 'dist' directory
docker push dockerimages.yinnut.com:15043/${SVC_NAME}:${VERSION}

echo "######################### deploy service ############################"
ANSIBLE_BASE_PATH="/home/dockerbuild/yinnut/kubernetes_contrib/ansible/"
cd ${ANSIBLE_BASE_PATH}
ansible all -i "localhost," -c local -m copy -a "src=${SVC_PATH}/dist/k8s/${SVC_NAME}-rc.yaml.j2 dest=roles/yinnut-services/templates/${SVC_NAME}/rc.yaml.j2 force=yes"
ansible all -i "localhost," -c local -m copy -a "src=${SVC_PATH}/dist/k8s/${SVC_NAME}-svc.yaml.j2 dest=roles/yinnut-services/templates/${SVC_NAME}/svc.yaml.j2 force=yes"

echo "########################## deploy service ###########################"

source ./ansible_dev.sh
INVENTORY=./inventory.dev ./setup.sh -e "svc_version=${VERSION}" -t ${SVC_NAME} -vv

echo "######################### clear directory /home/dockerbuild/${SVC_NAME} ###########################"
ansible all -i "localhost," -c local -m file -a "path=/home/dockerbuild/${SVC_NAME} state=absent"
