#!/bin/sh
CERTSDIR=/etc/kubernetes/ssl

create_namespace() {
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

  curl --cacert $CERTSDIR/ca.pem --cert $CERTSDIR/master-client.pem --key \
$CERTSDIR/master-client-key.pem -XPOST \
-d'{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system"}}' \
https://127.0.0.1/api/v1/namespaces
}

sudo chown root:root /etc/kubernetes/ssl/*.pem && \
sudo chown root:root /etc/ssl/etcd/*.pem && \
sudo systemctl daemon-reload && \
sudo systemctl enable kubelet && \
sudo systemctl start kubelet && \
sudo systemctl enable kube-apiserver && \
sudo systemctl start kube-apiserver && \
curl --cacert $CERTSDIR/ca.pem --cert $CERTSDIR/master-client.pem \
--key $CERTSDIR/master-client-key.pem -X PUT \
-d "value={\"Network\":\"10.2.0.0/16\",\"Backend\":{\"Type\":\"vxlan\"}}" \
https://127.0.0.1:2379/v2/keys/coreos.com/network/config && \
create_namespace && \
chmod +x /opt/bin/kubectl && \
echo "Creating DNS addon..." && \
kubectl create -f /tmp/dns-addon.yaml && \
echo "Creating Elasticsearch RC..." && \
kubectl create -f /tmp/es-controller.yaml && \
echo "Creating Elasticsearch service..." && \
kubectl create -f /tmp/es-service.yaml && \
echo "Creating Kibana RC..." && \
kubectl create -f /tmp/kibana-controller.yaml && \
echo "Creating Kibana service..." && \
kubectl create -f /tmp/kibana-service.yaml && \
echo "Creating Heapster controller..." && \
kubectl create -f /tmp/heapster-controller.yaml && \
echo "Creating Heapster service..." && \
kubectl create -f /tmp/heapster-service.yaml && \
echo "Creating InfluxDB-Grafana controller..." && \
kubectl create -f /tmp/influxdb-grafana-controller.yaml && \
echo "Creating InfluxDB Service..." && \
kubectl create -f /tmp/influxdb-service.yaml && \
echo "Creating Grafana Service..." && \
kubectl create -f /tmp/grafana-service.yaml && \
echo "Creating Quay secret..." && \
kubectl create -f /tmp/quay-io-secret.yaml
