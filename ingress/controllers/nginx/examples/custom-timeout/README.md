This example shows how is possible to create a custom configuration for a particular timeout associated with an Ingress rule.

```
echo "
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: echoheaders
  annotations:
    ingress.kubernetes.io/proxy-read-timeout: "30"
spec:
  rules:
  - host: foo.bar.com
    http:
      paths:
      - path: /
        backend:
          serviceName: echoheaders
          servicePort: 80
" | kubectl create -f -
```

Check the annotation is present in the Ingress rule:
```
kubectl get ingress echoheaders -o yaml
```

Check the NGINX configuration is updated using kubectl or the status page:

```
$ kubectl exec nginx-ingress-controller-v1ppm cat /etc/nginx/nginx.conf
```

```
....
    location / {
    
        proxy_read_timeout 30s;
    
    }
....
```
