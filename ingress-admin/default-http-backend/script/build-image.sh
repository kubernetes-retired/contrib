#!/bin/bash
#
# The script builds default-http-backend component container, see usage function for how to run
# the script. After build completes, following container will be built, i.e.
#   caicloud/default-http-backend:${IMAGE_TAG}
#
# By default, IMAGE_TAG is latest.

set -o errexit
set -o nounset
set -o pipefail

ROOT=$(dirname "${BASH_SOURCE}")/..

function usage {
  echo -e "Usage:"
  echo -e "  ./build-image.sh [tag]"
  echo -e ""
  echo -e "Parameter:"
  echo -e " tag\tDocker image tag, treated as default-http-backend release version. If provided,"
  echo -e "    \tthe tag must be the form of vA.B.C, where A, B, C are digits, e.g."
  echo -e "    \tv1.0.1. If not provided, it will build images with tag 'latest'"
  echo -e ""
  echo -e "Environment variable:"
  echo -e " PUSH_TO_REGISTRY     \tPush images to caicloud registry or not, options: Y or N. Default value: ${PUSH_TO_REGISTRY}"
}

# -----------------------------------------------------------------------------
# Parameters for building docker image, see usage.
# -----------------------------------------------------------------------------
# Decide if we need to push the new images to caicloud registry.
PUSH_TO_REGISTRY=${PUSH_TO_REGISTRY:-"N"}

# Find image tag version, the tag is considered as release version.
if [[ "$#" == "1" ]]; then
  if [[ "$1" == "help" || "$1" == "--help" || "$1" == "-h" ]]; then
    usage
    exit 0
  else
    IMAGE_TAG=${1}
  fi
else
  IMAGE_TAG="latest"
fi

echo "+++++ Start building default-http-backend"

cd ${ROOT}

# Build default-http-backend binary.
docker run --rm -v `pwd`:/go/src/k8s.io/contrib/ingress-admin/default-http-backend index.caicloud.io/caicloud/golang:1.6 sh -c "cd /go/src/k8s.io/contrib/ingress-admin/default-http-backend && go build -race ."
# Build default-http-backend container.
docker build -t caicloud/default-http-backend:${IMAGE_TAG} .
docker tag caicloud/default-http-backend:${IMAGE_TAG} index.caicloud.io/caicloud/default-http-backend:${IMAGE_TAG}

cd - > /dev/null

# Decide if we need to push images to caicloud registry.
if [[ "$PUSH_TO_REGISTRY" == "Y" ]]; then
  echo ""
  echo "+++++ Start pushing default-http-backend"
  docker push index.caicloud.io/caicloud/default-http-backend:${IMAGE_TAG}
fi

echo "Successfully built docker image caicloud/default-http-backend:${IMAGE_TAG}"
echo "Successfully built docker image index.caicloud.io/caicloud/default-http-backend:${IMAGE_TAG}"

# A reminder for creating Github release.
if [[ "$#" == "1" && $1 =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo -e "Finish building release ; if this is a formal release, please remember"
  echo -e "to create a release tag at Github at: https://github.com/caicloud/default-http-backend/releases"
fi