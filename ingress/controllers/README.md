# Ingress Controllers

Configuring a webserver or loadbalancer is harder than it should be. Most webserver configuration files are very similar. There are some applications that have weird little quirks that tend to throw a wrench in things, but for the most part you can apply the same logic to them and achieve a desired result. The Ingress resource embodies this idea, and an Ingress controller is meant to handle all the quirks associated with a specific "class" of Ingress (be it a single instance of a loadbalancer, or a more complicated setup of frontends that provide GSLB, DDoS protection etc).

## What is an Ingress Controller?

An Ingress Controller is a daemon, deployed as a Kubernetes Pod, that watches the ApiServer's `/ingresses` endpoint for updates to the [Ingress resource](https://github.com/kubernetes/kubernetes/blob/master/docs/user-guide/ingress.md). Its job is to satisfy requests for ingress.

## Writing an Ingress Controller

Writing an Ingress controller is simple. By way of example, the [nginx controller](nginx) does the following:
* Poll until apiserver reports a new Ingress
* Write the nginx config file based on a [go text/template](https://golang.org/pkg/text/template/).
* Reload nginx

By default nginx controller uses [this template](nginx/nginx.tmpl).

If you wish to use custom nginx template, please refer for this [example](nginx/examples/custom-template).

In this example let's use following simplified `nginx.conf` template:

```go
{{ $cfg := .cfg }}
daemon on;

events {
  worker_connections 1024;
}
http {
  {{range $name, $upstream := .upstreams}}
  upstream {{$upstream.Name}} {
      {{ if $cfg.enableStickySessions -}}
      sticky hash=sha1 httponly;
      {{ else -}}
      least_conn;
      {{- end }}
      {{ range $server := $upstream.Backends }}server {{ $server.Address }}:{{ $server.Port }} max_fails={{ $server.MaxFails }} fail_timeout={{ $server.FailTimeout }};
      {{ end }}
  }
  {{ end }}                                                                                                             
  # http://nginx.org/en/docs/http/ngx_http_core_module.html                                                             
  types_hash_max_size 2048;                                                                                             
  server_names_hash_max_size 512;                                                                                       
  server_names_hash_bucket_size 64;                                                                                     
{{ range $server := .servers }}                                                                                         
  server {                                                                                                              
    listen 80;                                                                                                          
    server_name {{ $server.Name }};                                                                                     
{{- range $location := $server.Locations }}                                                                             
{{ $path := buildLocation $location }}                                                                                  
    location {{ $path }} {                                                                                              
      proxy_set_header Host $host;                                                                                      
      {{ buildProxyPass $location }}                                                                                    
    }{{end}}                                                                                                            
  }{{end}}                                                                                                              
                                                                                                                        
  # default server, including healthcheck                                                                               
  server {                                                                                                              
      listen 8080 default_server reuseport;                                                                             

      location /healthz {
          access_log off;
          return 200;
      }

      location /nginx_status {
          {{ if $cfg.enableVtsStatus -}}
          vhost_traffic_status_display;
          vhost_traffic_status_display_format html;
          {{ else }}
          access_log off;
          stub_status on;
          {{- end }}
      }

      location / {
          proxy_pass             http://upstream-default-backend;
      }
  }
}
```

You can take a similar approach to denormalize the Ingress to a [haproxy config](https://github.com/kubernetes/contrib/blob/master/service-loadbalancer/template.cfg) or use it to configure a cloud loadbalancer such as a [GCE L7](https://github.com/kubernetes/contrib/blob/master/ingress/controllers/gce/README.md).

All this is doing is:
* List Ingresses, optionally you can watch for changes (see [GCE Ingress controller](https://github.com/kubernetes/contrib/blob/master/ingress/controllers/gce/controller/controller.go) for an example)
* Executes the template and writes results to `/etc/nginx/nginx.conf`
* Reloads nginx

You can deploy this controller to a Kubernetes cluster by [creating an RC](nginx/rc.yaml). After doing so, if you were to create an Ingress such as:
```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: test
spec:
  rules:
  - host: foo.bar.com
    http:
      paths:
      - path: /foo
        backend:
          serviceName: foosvc
          servicePort: 80
  - host: bar.baz.com
    http:
      paths:
      - path: /bar
        backend:
          serviceName: barsvc
          servicePort: 80
```

Where `foosvc` and `barsvc` are 2 services running in your Kubernetes cluster. The controller would satisfy the Ingress by writing a configuration file to `/etc/nginx/nginx.conf`:

```nginx
daemon on;

events {
  worker_connections 1024;
}
http {
  
  upstream default-barsvc-80 {
      least_conn;
      server 127.0.0.1:8181 max_fails=0 fail_timeout=0;
      
  }
  
  upstream default-foosvc-80 {
      least_conn;
      server 127.0.0.1:8181 max_fails=0 fail_timeout=0;
      
  }
  
  upstream upstream-default-backend {
      least_conn;
      server 10.100.75.10:8080 max_fails=0 fail_timeout=0;
      
  }
  
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  server {
    listen 80;
    server_name _;

    location / {
      proxy_set_header Host $host;
      proxy_pass http://upstream-default-backend;
    }
  }
  server {
    listen 80;
    server_name bar.baz.com;

    location /bar {
      proxy_set_header Host $host;
      proxy_pass http://default-barsvc-80;
    }

    location / {
      proxy_set_header Host $host;
      proxy_pass http://upstream-default-backend;
    }
  }
  server {
    listen 80;
    server_name foo.bar.com;

    location /foo {
      proxy_set_header Host $host;
      proxy_pass http://default-foosvc-80;
    }

    location / {
      proxy_set_header Host $host;
      proxy_pass http://upstream-default-backend;
    }
  }

  # default server, including healthcheck
  server {
      listen 8080 default_server reuseport;

      location /healthz {
          access_log off;
          return 200;
      }
     
      location /nginx_status {
          
          access_log off;
          stub_status on;
      }

      location / {
          proxy_pass             http://upstream-default-backend;
      }
  }
}
```

And you can reach the `/foo` and `/bar` endpoints on the publicIP of the VM the nginx-ingress pod landed on.
```
$ kubectl get pods -o wide
NAME                  READY     STATUS    RESTARTS   AGE       NODE
nginx-ingress-tk7dl   1/1       Running   0          3m        e2e-test-beeps-minion-15p3

$ kubectl get nodes e2e-test-beeps-minion-15p3 -o yaml | grep -i externalip -B 1
  - address: 104.197.203.179
    type: ExternalIP

$ curl --resolve foo.bar.com:80:104.197.203.179 foo.bar.com/foo
```

## Future work

This section can also bear the title "why anyone would want to write an Ingress controller instead of directly configuring Services". There is more to Ingress than webserver configuration. *Real* HA usually involves the configuration of gateways and packet forwarding devices, which most cloud providers allow you to do through an API. See the GCE Loadbalancer Controller, which is deployed as a [cluster addon](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/cluster-loadbalancing/glbc) in GCE and GKE clusters for more advanced Ingress configuration examples. Post 1.2 the Ingress resource will support at least the following:
* More TLS options (SNI, re-encrypt etc)
* L4 and L7 loadbalancing (it currently only supports HTTP rules)
* Ingress Rules that are not limited to a simple path regex (eg: redirect rules, session persistence)

And is expected to be the way one configures a "frontends" that handle user traffic for a Kubernetes cluster.
