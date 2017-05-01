#!/bin/sh
curl -s -L -o cfssljson https://pkg.cfssl.org/R1.1/cfssljson_linux-amd64
chmod +x cfssljson

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd ${DIR}/assets/certificates

IP_ADDRESS=$(ip route get 8.8.8.8 | awk '{print $NF;exit}')
EXT_IP_ADDRESS=$(curl -H "Metadata-Flavor: Google" \
http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip)
QUALIFIED_HOSTNAME=$(hostname)
HOSTNAME=${QUALIFIED_HOSTNAME%%.*}
cat <<EOF > worker.json
{
  "CN": "node.staging.realtimemusic.com",
  "hosts": [
    "127.0.0.1",
    "${IP_ADDRESS}",
    "${EXT_IP_ADDRESS}",
    "${HOSTNAME}",
    "${QUALIFIED_HOSTNAME}"
  ],
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "DE",
      "L": "Germany",
      "ST": ""
    }
  ]
}
EOF

docker run -v `pwd`:/certs cfssl/cfssl gencert -ca=/certs/ca.pem -ca-key=/certs/ca-key.pem -config=/certs/ca-config.json -profile=client-server /certs/worker.json | ../../../cfssljson -bare worker-client
