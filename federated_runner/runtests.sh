#!/bin/bash -ex

##=============================================================
# Job settings
export JOB_NAME="kubernetes-e2e-aws"
export E2E_MIN_STARTUP_PODS="1"
export MASTER_SIZE="m3.large"
export NODE_SIZE="m3.large"
export NUM_NODES="3"
export GINKGO_TEST_ARGS="--ginkgo.skip=\[Slow\]|\[Serial\]|\[Disruptive\]|\[Flaky\]|\[Feature:.+\]"
export GINKGO_PARALLEL="y"

##=============================================================
# AWS Settings
export KUBERNETES_PROVIDER="aws"
export KUBE_AWS_ZONE="us-west-2a"
#export PROJECT="k8s-jkns-e2e-aws"
export AWS_CONFIG_FILE=~/.aws/credentials
export AWS_SSH_KEY=~/.ssh/kube_aws_rsa
export KUBE_SSH_USER=admin
# This is needed to be able to create PD from the e2e test
export AWS_SHARED_CREDENTIALS_FILE=~/.aws/credentials

# GCE variables
#export INSTANCE_PREFIX="${E2E_NAME:-jenkins-e2e}"
#export KUBE_GCE_NETWORK="${E2E_NAME:-jenkins-e2e}"
#export KUBE_GCE_INSTANCE_PREFIX="${E2E_NAME:-jenkins-e2e}"
#export GCE_SERVICE_ACCOUNT=$(gcloud auth list 2> /dev/null | grep active | cut -f3 -d' ')

##=============================================================
# Global settings
export AWS_PROFILE=kubernetes-e2e
export JENKINS_GCS_LOGS_PATH=gs://kopeio-kubernetes-e2e/logs

##=============================================================
# System settings (emulate jenkins)
export USER=root
export WORKSPACE=$HOME
# Nothing should want Jenkins $HOME
export HOME=${WORKSPACE}
export BUILD_NUMBER=`date -u +%Y%m%d%H%M%S`
export JENKINS_HOME=${HOME}


# Assume we're upping, testing, and downing a cluster
export E2E_UP="${E2E_UP:-true}"
export E2E_TEST="${E2E_TEST:-true}"
export E2E_DOWN="${E2E_DOWN:-true}"

# Skip gcloud update checking
export CLOUDSDK_COMPONENT_MANAGER_DISABLE_UPDATE_CHECK=true


##=============================================================

branch=master

build_dir=${JENKINS_HOME}/jobs/${JOB_NAME}/builds/${BUILD_NUMBER}/
rm -rf ${build_dir}
mkdir -p ${build_dir}/workspace

cd ${build_dir}/workspace

# Sanity check
#gsutil ls ${JENKINS_GCS_LOGS_PATH}

exit_code=0
SECONDS=0 # magic bash timer variable
curl -fsS --retry 3  "https://raw.githubusercontent.com/kubernetes/kubernetes/master/hack/jenkins/e2e-runner.sh" > /tmp/e2e.sh
chmod +x /tmp/e2e.sh

set -e
/tmp/e2e.sh 2>&1 | tee ${build_dir}/build-log.txt
exit_code=${PIPESTATUS[0]}
duration=$SECONDS
set +e

if [[ ${exit_code} == 0 ]]; then
  success="true"
else
  success="false"
fi

version=`cat kubernetes/version`

gcs_acl="public-read"
gcs_job_path="${JENKINS_GCS_LOGS_PATH}/${JOB_NAME}"
gcs_build_path="${gcs_job_path}/${BUILD_NUMBER}"

gsutil -q cp -a "${gcs_acl}" -z txt "${build_dir}/build-log.txt" "${gcs_build_path}/"

curl -fsS --retry 3 "https://raw.githubusercontent.com/kubernetes/kubernetes/master/hack/jenkins/upload-to-gcs.sh" | bash -


curl -fsS --retry 3 "https://raw.githubusercontent.com/kubernetes/kubernetes/master/hack/jenkins/upload-finished.sh" > upload-finished.sh
chmod +x upload-finished.sh

if [[ ${exit_code} == 0 ]]; then
  ./upload-finished.sh SUCCESS
else
  ./upload-finished.sh UNSTABLE
fi

