#! /bin/bash

# This testlib is for dev purposes.

K=${KUBECTL:-kubectl}
GIT_ROOT=$(git rev-parse --show-cdup)

# curlNodePort gets all NodePorts for all ports of Services matching label app=$app and curls 1 node's external ip.
# Eg: `curlNodePort nginxsvc` will curl ip-of-first-node:nodePort-of-nginxsvc
function curlNodePort {
  port=`"${K}" get svc -l app=$1 -o template --template='{{range $.items}}{{range .spec.ports}}{{.nodePort}} {{end}}{{end}}'`
  node=`"${K}" get nodes --template='{{range .items}}{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}} {{end}}{{end}}{{end}}' | awk '{print $1}'`
  for p in "${port}"; do
      for n in "${node}"; do
          echo curling "${n}":"${p}"
          # TODO: check return code, i.e curl -s -o /dev/null -w "%{http_code}"
          curl $n:$p
      done
  done
}

# curl HTTPSWithHost performs a HTTPS curl using the cacert.
# $1: hostname
# $2: https port, usually 443. This is the hostPort/nodePort of the frontend.
# $3: public ip of the frontned, or any node in cluster if frontend is a nodePort svc.
# $4: path to a cacert. The CNAME in the cacert must match the hostname ($1).
# Eg: `curlHTTPSWithHost nginx 8082 104.197.79.157 nginx.crt` will result in
# curl --resolve nginx:8082:104.197.79.157 https://nginx:8082 --cacert nginx.crt
function curlHTTPSWithHost {
    echo curl --resolve $1:$2:$3 https://$1:$2 --cacert $4
    curl --resolve $1:$2:$3 https://$1:$2 --cacert $4
}

# waitForPods waits till all pods with label app=$app leave Pending or Terminating.
# Eg: `waitForPods frontend` will wait till all pods with the app=frontend label
# have left Terminating or Pending. This is obviously not the best way to do this
# but it'll do for now.
function waitForPods {
    # TODO: Count and wait for Running.
    while [ `"${K}" get pods --template='{{range .items}}{{if .metadata.deletionTimestamp }}{{.metadata.name}}{{end}}{{end}}'` ]; do
        echo waiting for $1 pods to leave Terminating
    done
    while [ `"${K}" get pods --template='{{range .items}}{{if eq .status.phase "Pending" }}{{.metadata.name}}{{end}}{{end}}'` ]; do
        echo waiting for $1 pods to leave Pending
    done
    echo $1 pods no longer pending
}

# cleanup deletes the app.
# Eg: `cleanup frontend` will delete all rc,svc,pods with label app=frontend.
function cleanup {
    echo Cleaning up $1
    "${K}" delete rc,svc,secrets -l app=$1
}

# checkCluster tries to retrieve cluster-info.
# Eg: checkCluster will execute kubectl cluster-info and exist on non-zero return.
function checkCluster {
    "${K}" cluster-info
    if [ $? -ne 0 ]; then
        echo cluster is down
        exit 1
    fi
}


# makeCerts makes certificates applying the given hostnames as CNAMEs
# $1 Name of the app that will use this secret, applied as a app= label
# $2... hostnames as described below
# Eg: makeCerts nginxsni nginx1 nginx2 nginx3
# Will generate nginx{1,2,3}.crt,.key,.json file in cwd. It's upto the caller
# to execute kubectl -f on the json file. The secret will have a label of
# app=nginxsni, so you can delete it via the cleanup function.
function makeCerts {
    local label=$1
    shift
    for h in ${@}; do
        if [ ! -f $h.json ] || [ ! -f $h.crt ] || [ ! -f $h.key ]; then
            printf "\nCreating new secrets for $h, will take ~30s\n\n"
            local cert=$h.crt key=$h.key host=$h secret=$h.json cname=$h
            if [ $h == "wildcard" ]; then
                cname=*.$h.com
            fi

            # Generate crt and key
        	openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
                    -keyout "${key}" -out "${cert}" -subj "/CN=${cname}/O=${cname}"

            # Create secret.json
            CGO_ENABLED=0 GOOS=linux godep go run -a -installsuffix cgo \
                    -ldflags '-w' "${GIT_ROOT}"Ingress/hack/make_secret.go -crt "${cert}" -key "${key}" \
                    -name "${host}" -app "${label}" > "${host}".json

            # Create secret with API Server
            "${K}" create -f "${host}".json

        else
            echo WARNING: Secret for $h already found, make clean to remove
            exit 1
        fi
    done
}

# getNodeIPs echoes a list of node ips for all pods matching the name=label.
# $1 is the string used in the name= label of the pod.
# Eg: getNodeIPs frontend will get all public node ips of pods with name=frontend.
function getNodeIPs {
    nodes=`"${K}" get pod -l name=$1 --template='{{range .items}}{{if eq .status.phase "Running"}}{{.spec.nodeName}} {{end}}{{end}}'`
    for n in ${nodes[*]}; do
        echo `"${K}" get nodes $n --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}} {{end}}{{end}}'`
    done
}
