# Test Ingresses

Used services are created [here](../backends/README.md)
Balancer is created [here](../balancer/README.md)

To test all incresses execute

```
kubectl create -f all.yaml
```  

To test the ingresses you may need to add `Host` header to the requests

- Requests to `game.stackpoint.io` will be sent to the `game2048` service
- Requests to `foo.stackpoint.io` with path `/bar1`or `/bar2`will be sent to `echoheaders` service
- Requests not matching any of the previous rules will be sent to `default-backend` service
