#!/bin/sh
CERTSDIR=/etc/kubernetes/ssl

function create_namespace() {
  wait="true"
  while [[ $wait = "true" ]]
  do
    curl -s --cacert $CERTSDIR/ca.pem --cert $CERTSDIR/master-client.pem --key \
$CERTSDIR/master-client-key.pem https://127.0.0.1/version
    if [[ $? = 0 ]]
    then
      wait="false"
    else
      echo Waiting for API server to come up...
      sleep 1
    fi
  done

  echo Creating kube-system namespace...
  curl --cacert $CERTSDIR/ca.pem --cert $CERTSDIR/master-client.pem --key \
$CERTSDIR/master-client-key.pem -H "Content-Type: application/json" -XPOST \
-d'{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system"}}' \
https://127.0.0.1/api/v1/namespaces
}

create_namespace && \
echo "Creating DNS addon..." && \
/opt/bin/kubectl create -f /tmp/dns-addon.yaml && \
echo "Creating Heapster controller..." && \
/opt/bin/kubectl create -f /tmp/heapster-controller.yaml && \
echo "Creating Heapster service..." && \
/opt/bin/kubectl create -f /tmp/heapster-service.yaml && \
echo "Creating Dashboard controller..." && \
/opt/bin/kubectl create -f /tmp/dashboard-controller.yaml && \
echo "Creating Dashboard service..." && \
/opt/bin/kubectl create -f /tmp/dashboard-service.yaml && \
echo "Creating Quay secret..." && \
/opt/bin/kubectl create -f /tmp/quay-io-secret.yaml
echo "Creating GLBC controller..." && \
/opt/bin/kubectl create -f /tmp/glbc-controller.yaml
echo "Creating GLBC service..." && \
/opt/bin/kubectl create -f /tmp/glbc-service.yaml
