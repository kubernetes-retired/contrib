# Test Ingresses

Used services are created [here](../backends/README.md)
Balancer is created [here](../balancer/README.md)

## SSL (SNI)

We will demo using SSL certificates with a single IP.
The example will create two TLS secured sites, and one default backend also with TLS

Execute:
```
kubectl create -f ssl-scenarios.yaml
```

To test this case you need to resolve to the requests. (Not only add to host header, since host header isn't used by SNI)

- Requests to `https://b.stackpoint.io` will be sent to the `game2048` service
- Requests to `https://b.b.stackpoint.io` will be sent to `echoheaders` service
- Requests to `https://<balancer-ip>` will be sent to `default-backend` service


To test the ingresses you may need to add `Host` header to the requests

- Requests to `game.stackpoint.io` will be sent to the `game2048` service
- Requests to `foo.stackpoint.io` with path `/bar1`or `/bar2`will be sent to `echoheaders` service
- Requests not matching any of the previous rules will be sent to `default-backend` service



## How to create self-signed SSL Certs


Generate self-signed test certificate

```
$ openssl req -newkey rsa:2048 -nodes -keyout cert.key -x509 -days \
   -out cert.crt -subj "/C=US/ST=Seattle/O=Stackpoint/CN=secure.stackpoint.io"

```

Create secrets withbase64 encoded certificates

```
$ echo "
apiVersion: v1
kind: Secret
metadata:
  name: game
data:
  tls.crt: `base64 ./cert.crt`
  tls.key: `base64 ./cert.key`
" | kubectl create -f -
```
