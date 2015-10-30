#!/bin/bash

# $1 = the kubernetes context (specified in kubeconfig)
# $2 = directory that contains your kubernetes files to deploy

DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
CONTEXT=$1
DEPLOYDIR=$2

#make sure we have the kubectl comand
chmod +x $DIR/ensure-kubectl.sh
$DIR/ensure-kubectl.sh

#set config context
~/.kube/kubectl config use-context ${CONTEXT}

#get user password and api ip from config data
export kubepass=`(~/.kube/kubectl config view -o json | jq ' { mycontext: .["current-context"], contexts: .contexts[], users: .users[], clusters: .clusters[]}' | jq 'select(.mycontext == .contexts.name) | select(.contexts.context.user == .users.name) | select(.contexts.context.cluster == .clusters.name)' | jq .users.user.password | tr -d '\"')`

export kubeuser=`(~/.kube/kubectl config view -o json | jq ' { mycontext: .["current-context"], contexts: .contexts[], users: .users[], clusters: .clusters[]}' | jq 'select(.mycontext == .contexts.name) | select(.contexts.context.user == .users.name) | select(.contexts.context.cluster == .clusters.name)' | jq .users.user.username | tr -d '\"')`

export kubeurl=`(~/.kube/kubectl config view -o json | jq ' { mycontext: .["current-context"], contexts: .contexts[], users: .users[], clusters: .clusters[]}' | jq 'select(.mycontext == .contexts.name) | select(.contexts.context.user == .users.name) | select(.contexts.context.cluster == .clusters.name)' | jq .clusters.cluster.server | tr -d '\"')`

export kubenamespace=`(~/.kube/kubectl config view -o json | jq ' { mycontext: .["current-context"], contexts: .contexts[]}' | jq 'select(.mycontext == .contexts.name)' | jq .contexts.context.namespace | tr -d '\"')`

export kubeip=`(echo $kubeurl | sed 's~http[s]*://~~g')`

export https=`(echo $kubeurl | awk 'BEGIN { FS = ":" } ; { print $1 }')`

set -x

#print some useful data for folks to check on their service later
echo "Deploying service to ${https}://${kubeuser}:${kubepass}@${kubeip}/api/v1/proxy/namespaces/${kubenamespace}/services/${SERVICENAME}"
echo "Monitor your service at ${https}://${kubeuser}:${kubepass}@${kubeip}/api/v1/proxy/namespaces/kube-system/services/kibana-logging/?#/discover?_a=(columns:!(_source),filters:!(),index:'logstash-*',interval:auto,query:(query_string:(analyze_wildcard:!t,query:'tag:kubernetes.${SERVICENAME}*')))"

# delete service (does nothing if service does not exist already)
for f in ${DEPLOYDIR}/*.yaml; do envsubst < $f > kubetemp.yaml; cat kubetemp.yaml; ~/.kube/kubectl delete --namespace=${kubenamespace} -f kubetemp.yaml || /bin/true; done

# create service (does nothing if the service already exists)
for f in ${DEPLOYDIR}/*.yaml; do envsubst < $f > kubetemp.yaml; ~/.kube/kubectl create --namespace=${kubenamespace} -f kubetemp.yaml || /bin/true; done

# wait for services to start
sleep 30

# try to hit the endpoint
curl -k ${https}://${kubeuser}:${kubepass}@${kubeip}/api/v1/proxy/namespaces/${kubenamespace}/services/${SERVICENAME}/

# perform a rolling update. $CIRCLE_SHA1 can be used to tag, push and run your container
#~/.kube/kubectl rolling-update ${SERVICENAME} --image=${DOCKER_REGISTRY}/${CONTAINER1}:$4 --namespace=$2  || /bin/true