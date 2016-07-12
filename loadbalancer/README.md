# High Available Multi-Backend Load balancer

The project implements a load balancer controller that will provide a high available and load balancing access to HTTP and TCP kubernetes applications. It also provide SSL support for http apps.

Our goal is to have this controller listen to ingress events, rather than config map for generating config rules. Currently, this controller watches for configmap resources
to create and configure backend. Eventually, this will be changed to watch for ingress resource instead. This feature is still being planned in kubernetes since the current version
of ingress does not support layer 4 routing.

This controller is designed to easily integrate and create different load balancing backends. From software, hardware to cloud loadbalancer. Our initial featured backends are Openstack LBaaS v2 (Octavia) and Nginx (via configmaps).

In the case for software loadbalacer, this controllers work with loadbalancer-controller daemons which are deployed across nodes which will servers as high available loadbalacers. These daemon controllers use keepalived and nginx to provide 
the high availability loadbalancing via the use of VIPs. The loadbalance controller will communicate with the daemons via a configmap resource.

**Note**: The daemon needs to run in priviledged mode and with `hostNetwork: true` so that it has access to the underlying node network. This is needed so that the VIP can be assigned to the node interfaces so that they are accessible externally.

## Examples

1. First we need to create the loadbalancer controller. You can specify the type of backend used for the loadbalancer via an environment variable. If using openstack loadbalancer, provide your Openstack information as an environment variables. The password is supplied via a secret resource. 
  ```
  $ kubectl create -f example/ingress-loadbalancer-rc.yaml
  ```

1. Create our sample app, which consists of a service and replication controller resource. If using cloud loadbalancer, make sure your application is deployed with `type: NodePort`:

  ```
  $ kubectl create -f examples/coffee-app.yaml
  ```

1. Create configmap for the sample app service. This will be used to configure the loadbalancer backend:
  ```
  $ kubectl create -f coffee-configmap.yaml
  ```

### Cloud Load Balancing (Openstack LBaaS V2)

1. An Openstack Loadbalancer should be triggered and configure. Wait until it is ACTIVE
  ```
  $ neutron lbaas-loadbalancer-list
  +--------------------------------------+-------------------------------------------+-------------+---------------------+----------+
  | id                                   | name                                      | vip_address | provisioning_status | provider |
  +--------------------------------------+-------------------------------------------+-------------+---------------------+----------+
  | ec6ff6ea-3143-4019-ba4e-d7d0873a945f | default-configmap-coffee-svc-loadbalancer | 10.0.0.81   | ACTIVE              | octavia  |
  +--------------------------------------+-------------------------------------------+-------------+---------------------+----------+
  ```

1. Curl the VIP to access the coffee app
  ```
  $ curl http://10.0.0.81
  <!DOCTYPE html>
  <html>
  <head>
  <title>Hello from NGINX!</title>
  <link href="data:image/png;base64,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" rel="icon" type="image/png" />

  <style>
      body {
          width: 35em;
          margin: 0 auto;
          font-family: Tahoma, Verdana, Arial, sans-serif;
      }
  </style>
  </head>
  <body>
  <h1>Hello!</h1>
  <h2>URI = /</h2>
  <h2>My hostname is coffee-rc-auqj8</h2>
  <h2>My address is 172.18.99.3:80</h2>
  </body>
  </html>
  ```

1. The apps are accessed via a nodePort in the K8 nodes which is in the range of 30000-32767. Make sure they are open in the nodes. Also make sure to open up any ports that bind to the load balancer, such as port 80 in this case.

### Software Loadbalancer using keepalived and nginx

1. Get the bind IP generated by the loadbalancer controller from the configmap.
```
$ kubectl get configmap configmap-coffee-svc -o yaml
apiVersion: v1
data:
  bind-ip: "10.0.0.10"
  bind-port: "80"
  namespace: default
  target-port: "80"
  target-service-name: coffee-svc
kind: ConfigMap
metadata:
  creationTimestamp: 2016-06-17T22:30:03Z
  labels:
    app: loadbalancer
  name: configmap-coffee-svc
  namespace: default
  resourceVersion: "157728"
  selfLink: /api/v1/namespaces/default/configmaps/configmap-coffee-svc
  uid: 08e12303-34db-11e6-87da-fa163eefe713
```

1. To get coffee:
```
  $ curl http://10.0.0.10
  <!DOCTYPE html>
  <html>
  <head>
  <title>Hello from NGINX!</title>
  <style>
      body {
          width: 35em;
          margin: 0 auto;
          font-family: Tahoma, Verdana, Arial, sans-serif;
      }
  </style>
  </head>
  <body>
  <h1>Hello!</h1>
  <h2>URI = /coffee</h2>
  <h2>My hostname is coffee-rc-mu9ns</h2>
  <h2>My address is 10.244.0.3:80</h2>
  </body>
  </html>
```
**Note**: Implementations are experimental and not suitable for using in production. This project is still in its early stage and many things are still in work in progress.
