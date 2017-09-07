# Pool of backends

Each yaml file here creates the example pods and services.

## All backends


```
kubectl create -f all.yaml
```  

### Service: echoheaders

Creates a service that returns the received headers.
- Pod port: 8080
- Svc port: 80

## Service: default-http-backend

Creates a service that servers a default 404 page
- Pod port: 8080
- Svc port: 80

## Service: game2048

Creates a service with the game 2048
- Pod port: 80
- Svc port: 80
