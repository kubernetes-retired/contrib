#!/bin/bash

set -vx

# a url that your ci system can hit to pull down your kube config file
export KUBEURL=http://

# contexts from your kubeconfig file that are used for deployment
export KUBECONTEXTQA=aws_kubernetes
export KUBECONTEXTPROD=aws_kubernetes2
# update this to the directory where your yaml\json files are for kubernetes relative to your project root directory
export KUBEDEPLOYMENTDIR=./kubeyaml
export BUILD=${BUILD_NUMBER}

# used for interpod and interservice communication
# Must be lowercase and <= 24 characters
# defaulted to job-branch for jenkins
export SERVICENAME=$(tr [A-Z] [a-z] <<< ${JOB_NAME:0:8})-$(tr [A-Z] [a-z] <<< ${GIT_BRANCH:0:15} | tr -d '_-' | sed 's/\//-/g')

# This uses the docker socket on the host instead of inside the container for caching\performance reasons
export DOCKER_HOST=unix:///var/run/docker.sock
# the docker repo
export DOCKER_REGISTRY=docker-sandbox.concurtech.net
# the docker container defaulted to job/branch for jenkins
export CONTAINER1=$(tr [A-Z] [a-z] <<< ${JOB_NAME:0:8})/$(tr [A-Z] [a-z] <<< ${GIT_BRANCH:0:15}| tr -d '_-' | sed 's/\//-/g')

#login to docker repo
#dockeruser and dockerpass are coming from a jenkins credential in this example
docker login -u ${dockeruser} -p ${dockerpass} -e jenkins@domain.com ${DOCKER_REGISTRY}

# build the container from the Dockerfile in the project
docker build -t ${DOCKER_REGISTRY}/${CONTAINER1} .

#tag the container
docker tag -f ${DOCKER_REGISTRY}/${CONTAINER1}:latest ${DOCKER_REGISTRY}/${CONTAINER1}:build${BUILD}

#push the two container tags to the repo
docker push ${DOCKER_REGISTRY}/${CONTAINER1}:build${BUILD}
docker push ${DOCKER_REGISTRY}/${CONTAINER1}:latest

#deploy to QA
chmod +x ./deploy/deploy-service.sh && ./deploy/deploy-service.sh ${KUBECONTEXTQA} ${KUBEDEPLOYMENTDIR}

#put integration tests here
echo "put integration tests here"

#deploy to production cluster
./deploy/deploy-service.sh ${KUBECONTEXTPROD} ${KUBEDEPLOYMENTDIR}

#put deployment verification tests here
echo "put deployment verification tests here"