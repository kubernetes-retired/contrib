#!/bin/bash -ex

JOB_NAME=$1

if [[ -z "${JOB_NAME}" ]]; then
  echo "Must pass <job_name>"
  exit 1
fi

export JOB_NAME
echo "JOB_NAME=${JOB_NAME}"
echo "Loading conf/${JOB_NAME}"

. conf/${JOB_NAME}

echo "Loading conf/cloud/${KUBERNETES_PROVIDER}"
. conf/cloud/${KUBERNETES_PROVIDER}

echo "Loading conf/site"
. conf/site

##=============================================================
# Global settings
export KUBE_GCS_RELEASE_BUCKET=kubernetes-release

# We download the binaries ourselves
# TODO: No way to tell e2e to use a particular release?
# TODO: Maybe download and then bring up the cluster?
export JENKINS_USE_EXISTING_BINARIES=y

# This actually just skips kube-up master detection
export KUBERNETES_CONFORMANCE_TEST=y

##=============================================================
# System settings (emulate jenkins)
export USER=root
export WORKSPACE=$HOME
# Nothing should want Jenkins $HOME
export HOME=${WORKSPACE}
export BUILD_NUMBER=`date -u +%Y%m%d%H%M%S`
export JENKINS_HOME=${HOME}

# We'll directly up & down the cluster
export E2E_UP="${E2E_UP:-false}"
export E2E_TEST="${E2E_TEST:-true}"
export E2E_DOWN="${E2E_DOWN:-false}"

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

# We need kubectl to write kubecfg from upup generate kubecfg
curl -fsS --retry 3  "https://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl" > /usr/local/bin/kubectl
chmod +x /usr/local/bin/kubectl

curl -fsS --retry 3  "https://kubeupv2.s3.amazonaws.com/upup/upup.tar.gz" > /tmp/upup.tar.gz
tar zxf /tmp/upup.tar.gz -C /opt

if [[ ! -e ${AWS_SSH_KEY} ]]; then
  echo "Creating ssh key ${AWS_SSH_KEY}"
  ssh-keygen -N "" -t rsa -f ${AWS_SSH_KEY}
fi

if [[ "${KUBERNETES_PROVIDER}" == "aws" ]]; then
  # Because we generate a new keypair every time, the keys won't match unless we delete if it already exists
  aws ec2 delete-key-pair --key-name kubernetes.${JOB_NAME}.${DNS_DOMAIN} || true
fi

function fetch_tars_from_gcs() {
    local -r bucket="${1}"
    local -r build_version="${2}"
    echo "Pulling binaries from GCS; using server version ${bucket}/${build_version}."
    gsutil -mq cp \
        "gs://${KUBE_GCS_RELEASE_BUCKET}/${bucket}/${build_version}/kubernetes.tar.gz" \
        "gs://${KUBE_GCS_RELEASE_BUCKET}/${bucket}/${build_version}/kubernetes-test.tar.gz" \
        .
}

function unpack_binaries() {
    md5sum kubernetes*.tar.gz
    tar -xzf kubernetes.tar.gz
    tar -xzf kubernetes-test.tar.gz
}


fetch_tars_from_gcs release ${KUBERNETES_VERSION}
unpack_binaries

# Clean up everything when we're done
function finish {
  /opt/upup/upup delete cluster \
                 --name ${JOB_NAME}.${DNS_DOMAIN} \
                 --region ${AWS_REGION} \
                 --yes   2>&1 | tee -a ${build_dir}/build-log.txt
}
trap finish EXIT

set -e

# Bring up cluster
pushd /opt/upup
/opt/upup/cloudup --name ${JOB_NAME}.${DNS_DOMAIN} \
                  --cloud ${KUBERNETES_PROVIDER} \
                  --zones ${NODE_ZONES} \
                  --node-size ${NODE_SIZE} \
                  --master-size ${MASTER_SIZE} \
                  --ssh-public-key ${AWS_SSH_KEY}.pub \
                  --state ${WORKSPACE}/state \
                  --kubernetes-version ${KUBERNETES_VERSION} \
                  --v=4 --logtostderr 2>&1 | tee -a ${build_dir}/build-log.txt
exit_code=${PIPESTATUS[0]}
popd

# Write kubecfg file
if [[ ${exit_code} == 0 ]]; then
  attempt=0
  while true; do
    /opt/upup/upup kubecfg generate --name ${JOB_NAME}.${DNS_DOMAIN} \
                                    --cloud ${KUBERNETES_PROVIDER} \
                                    --state ${WORKSPACE}/state 2>&1 | tee -a ${build_dir}/build-log.txt
    exit_code=${PIPESTATUS[0]}

    if [[ ${exit_code} == 0 ]]; then
      break
    fi
    if (( attempt > 60 )); then
      echo "Unable to generate kubecfg within 10 minutes (master did not launch?)"
      break
    fi
    attempt=$(($attempt+1))
    sleep 10
  done
fi

# Wait for kubectl to begin responding (at least master up)
if [[ ${exit_code} == 0 ]]; then
  attempt=0
  while true; do
    kubectl get nodes --show-labels  2>&1 | tee -a ${build_dir}/build-log.txt
    exit_code=${PIPESTATUS[0]}

    if [[ ${exit_code} == 0 ]]; then
      # TODO: remove this
      echo "API responded; waiting 120 seconds for DNS to settle"
      sleep 120
      break
    fi
    if (( attempt > 60 )); then
      echo "Unable to connect to API 10 minutes (master did not launch?)"
      break
    fi
    attempt=$(($attempt+1))
    sleep 10
  done
fi


# Run e2e tests
if [[ ${exit_code} == 0 ]]; then
  /tmp/e2e.sh 2>&1 | tee -a ${build_dir}/build-log.txt
  exit_code=${PIPESTATUS[0]}
fi

# Try to clean up normally so it goes into the logs
# (we have an exit hook for abnormal termination, but that does not get logged)
finish

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

