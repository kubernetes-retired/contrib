# Special tests

Used services are created [here](../backends/README.md)
Balancer is created [here](../balancer/README.md)

## Shadow Domains

Ingress controller should allow using domains with shared suffixs:

- `a.stackpoint.io`
- `a.a.stackpoint.io`
- `a.a.a.stackpoint.io`

each of them should be captured only by their own rule.

To test this case execute:

```
kubectl create -f nate-shadow-domains.yaml
```

To test this case you need to add `Host` header to the requests

- Requests to `a.stackpoint.io` will be sent to the `game2048` service
- Requests to `a.a.stackpoint.io` will be sent to `default-backend` service
- Requests to `a.a.a.stackpoint.io` will be sent to `echoheaders` service

## SSL (SNI)

We will demo using SSL certificates with a single IP.
The example will create two TLS secured sites, and one default backend also with TLS

Execute:
```
kubectl create -f ssl-scenarios.yaml
```

To test this case you need to add `Host` header to the requests

- Requests to `https://b.stackpoint.io` will be sent to the `game2048` service
- Requests to `https://b.b.stackpoint.io` will be sent to `echoheaders` service
- Requests to `https://<balancer-ip>` will be sent to `default-backend` service
