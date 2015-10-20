# Haproxy HTTPS example

See test.sh for example usage.

## Running through docker

```shell
$ cat haproxyhttps.crt haproxyhttps.key > haproxyhttps.key
$ docker run -it -v "/tmp/haproxyhttps.pem:/etc/haproxy/ssl/haproxy.pem" -p 8082:443 bprashanth/haproxyhttps:0.0
```
