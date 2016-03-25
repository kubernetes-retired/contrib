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
$CERTSDIR/master-client-key.pem -XPOST \
-d'{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system"}}' \
https://127.0.0.1/api/v1/namespaces
}

echo Configuring Flannel network...
curl --cacert $CERTSDIR/ca.pem --cert $CERTSDIR/master-client.pem \
--key $CERTSDIR/master-client-key.pem -X PUT \
-d "value={\"Network\":\"10.2.0.0/16\",\"Backend\":{\"Type\":\"vxlan\"}}" \
https://127.0.0.1:2379/v2/keys/coreos.com/network/config && \
create_namespace && \
echo "Creating DNS addon..." && \
/opt/bin/kubectl create -f /tmp/dns-addon.yaml && \
echo "Creating Elasticsearch RC..." && \
/opt/bin/kubectl create -f /tmp/es-controller.yaml && \
echo "Creating Elasticsearch service..." && \
/opt/bin/kubectl create -f /tmp/es-service.yaml && \
echo "Creating Kibana RC..." && \
/opt/bin/kubectl create -f /tmp/kibana-controller.yaml && \
echo "Creating Kibana service..." && \
/opt/bin/kubectl create -f /tmp/kibana-service.yaml && \
echo "Creating Heapster controller..." && \
/opt/bin/kubectl create -f /tmp/heapster-controller.yaml && \
echo "Creating Heapster service..." && \
/opt/bin/kubectl create -f /tmp/heapster-service.yaml && \
echo "Creating InfluxDB-Grafana controller..." && \
/opt/bin/kubectl create -f /tmp/influxdb-grafana-controller.yaml && \
echo "Creating InfluxDB Service..." && \
/opt/bin/kubectl create -f /tmp/influxdb-service.yaml && \
echo "Creating Grafana Service..." && \
/opt/bin/kubectl create -f /tmp/grafana-service.yaml && \
echo "Creating Quay secret..." && \
/opt/bin/kubectl create -f /tmp/quay-io-secret.yaml
