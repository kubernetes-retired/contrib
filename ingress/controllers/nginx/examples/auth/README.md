
This example shows how to add authentication in an Ingress rule using a secret that contains a file generated with `htpasswd`.


```
$ htpasswd -c auth foo
New password: <bar>
New password:
Re-type new password:
Adding password for user foo
```

```
$ kubectl create secret generic basic-auth --from-file=auth
secret "basic-auth" created
```

```
$ kubectl get secret basic-auth -o yaml
apiVersion: v1
data:
  auth: Zm9vOiRhcHIxJE9GRzNYeWJwJGNrTDBGSERBa29YWUlsSDkuY3lzVDAK
kind: Secret
metadata:
  name: basic-auth
  namespace: default
type: Opaque
```

```
echo "
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: ingress-with-auth
  annotations:
    # type of authentication
    ingress.kubernetes.io/auth-type: basic
    # name of the secret that contains the user/password definitions
    ingress.kubernetes.io/auth-secret: basic-auth
    # message to display with an appropiate context why the authentication is required
    ingress.kubernetes.io/auth-realm: "Authentication Required - foo"
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

```
$ curl -v http://10.2.29.4/ -H 'Host: foo.bar.com'
*   Trying 10.2.29.4...
* Connected to 10.2.29.4 (10.2.29.4) port 80 (#0)
> GET / HTTP/1.1
> Host: foo.bar.com
> User-Agent: curl/7.43.0
> Accept: */*
>
< HTTP/1.1 401 Unauthorized
< Server: nginx/1.10.0
< Date: Wed, 11 May 2016 05:27:23 GMT
< Content-Type: text/html
< Content-Length: 195
< Connection: keep-alive
< WWW-Authenticate: Basic realm="Authentication Required - foo"
<
<html>
<head><title>401 Authorization Required</title></head>
<body bgcolor="white">
<center><h1>401 Authorization Required</h1></center>
<hr><center>nginx/1.10.0</center>
</body>
</html>
* Connection #0 to host 10.2.29.4 left intact
```

```
$ curl -v http://10.2.29.4/ -H 'Host: foo.bar.com' -u 'foo:bar'
*   Trying 10.2.29.4...
* Connected to 10.2.29.4 (10.2.29.4) port 80 (#0)
* Server auth using Basic with user 'foo'
> GET / HTTP/1.1
> Host: foo.bar.com
> Authorization: Basic Zm9vOmJhcg==
> User-Agent: curl/7.43.0
> Accept: */*
>
< HTTP/1.1 200 OK
< Server: nginx/1.10.0
< Date: Wed, 11 May 2016 06:05:26 GMT
< Content-Type: text/plain
< Transfer-Encoding: chunked
< Connection: keep-alive
< Vary: Accept-Encoding
<
CLIENT VALUES:
client_address=10.2.29.4
command=GET
real path=/
query=nil
request_version=1.1
request_uri=http://foo.bar.com:8080/

SERVER VALUES:
server_version=nginx: 1.9.11 - lua: 10001

HEADERS RECEIVED:
accept=*/*
authorization=Basic Zm9vOmJhcg==
connection=close
host=foo.bar.com
user-agent=curl/7.43.0
x-forwarded-for=10.2.29.1
x-forwarded-host=foo.bar.com
x-forwarded-port=80
x-forwarded-proto=http
x-real-ip=10.2.29.1
BODY:
* Connection #0 to host 10.2.29.4 left intact
-no body in request-
```
