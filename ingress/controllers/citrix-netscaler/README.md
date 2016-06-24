HOWTO - Kubernetes with NetScaler Load Balancer
===============================================

------------
Introduction
------------

Kubernetes is an open source container cluster manager. This article describes how you can use a NetScaler specific ingress controller for Kubernetes to configure a NetScaler instance as a load balancer within the cluster.

A typical Kubernetes cluster deployment consists of Kube-master node and Kube-worker nodes. Before you begin configuring NetScaler MPX/VPX, the expectation is to have a Kubernetes cluster configured and running. 

Ensure that the following entities are available in the cluster along with Kubernetes: 
- Docker    (For containers)
- Flannel   (For networking)
- Etcd      (For configuration management)

In an up and running cluster, you can now configure the NetScaler MPX/VPX instance as a load balancer. The configuration of the MPX/VPX instance will be dynamic on the basis of the number of ingress entities configured, the services they belong to, and the endpoints serving the services. 

The NetScaler MPX/VPX instance acts as a load balancer by directly identifying the endpoints associated with the service that the ingress is supporting. Once identified, the NetScaler MPX/VPX instance load balances directly between the endpoint addresses bypassing the service Cluster-IP associated with the service. 


----------------
Getting the Code
----------------

The code can be obtained from the following git repository:
https://github.com/kubernetes/contrib.git

The area where the controller resides is:
`contrib/ingress/controllers/citrix-netscaler/`

The associated docker images can be obtained from dockerhub here:
https://hub.docker.com/u/adhamija/

To perform the compilation, you need the following: 
- Go language packages and dependencies
- Godeps dependency tool

Compilation should be perfomed in the citrix-netscaler area using the command:
`# godep go build`


---------------------------------
Sample application load balancing
---------------------------------

The sample application described here makes use of the guestbook example which has a web frontend based on php and backend based on redis. The configuration files and additional inforamtion is available here in the tree:
`contrib/ingress/controllers/citrix-netscaler/example/guestbook`

The example shown here has one Kubernetes master node and two worker nodes. The following revisions were used for the example: 
- Kubernetes    1.2.0-0.20
- Flannel       0.5.5-3 
- Etcd          2.0.9-1
- Docker        1.9.1-41
- OS            CentOS Linux release 7.2.1511 (Core)
- Netscaler     VPX NS11.1

Each node is connected to the management subnet and an internal subnet. Flannel running on each node utilizes a separate subnet. The management subnet is used for external access to the nodes. The internal subnet is used for internal traffic between nodes and flannel subnet is self-managed by flannel and the containers get assigned addresses from within this subnet.

As part of this example the following network address ranges are in use: 
- Management Subnet : 10.217.129.64/28
- Internal Subnet   : 10.11.50.0/24
- Flannel Subnet    : 10.254.0.0/16

Please have a running instance of NetScaler MPX/VPX available and configured with a management IP address before running the steps below.

### Step 1: 
The initial configuration required is for the NetScaler to be able to communicate with all the worker nodes ovr VXLAN. Flannel supports multiple kinds of networks but Netscaler supports VXLAN. The VNI is identified from the flannel configuration stored in etcd and configured on the NetScaler. 

A python script is made available that configures NetScaler with the VXLAN configuration and creates tunnels between it and each worker node. The tunnel endpoints reside on the internal subnet. Please note that the python sdk for Nitro is a prequisite for running the python sctip. The python script should be run from the kube-master node. 

- python pip is required for sdk installation
- Obtain sdk from NS GUI: `http://<NSIP>/menu/dw` -> NITRO API SDK for Python
- untar python sdk
- python setup.py install

###### Note: The version of python in use for example is 2.7.5

The script can subsequently be run as follows:

`python NSK8sConfig.py addvxlan <NSIP> <NS_USER> <NS_PASSWORD> <NS_TUNNEL_ENDPOINT_IP> <KUBE_MASTER_IP>`

`python NSK8sConfig.py addvxlan 10.217.129.75 nsroot nsroot 10.11.50.13 10.11.50.10`


To establish dynamic ARP exchange between the endpoints/pods running on each Kube-worker node over VXLAN, the following sysctl variables should be modified as shown on each Kube-worker node:

`# sysctl -w net.ipv4.conf.flannel/1.proxy_arp=1`

`# sysctl -w net.ipv4.ip_forward=1`

### Step 2:
The next configuration is to provide the presence of NetScaler to etcd so that it can be managed. We will allocate a subnet in the range controlled by flannel. This is done so that an address range is available to NetScaler to communicate with peers over flannel subnet and for flannel not to allocate that subnet to any other worker nodes that it manages resulting in address conflicts. 

The NetScaler subnet for Flannel can be chosen between the the range SubnetMin and SubnetMax obtainable from key `/flannel/network/config` in etcd. 

`# etcdctl --no-sync --peers http://<etcdIP>:<Port> get /flannel/network/config`

It is required that the interface of NetScaler connected to internal subnet be used for configuration here and the MAC address associated with the internal interface be provided. Any subnet subnet can be chosen for configuration as long as it is managed by flannel. 

Different flag in the previously stated python script provides this functionality:

`# python NSK8sConfig.py addmac <KUBE_MASTER_IP> <NS_TUNNEL_ENDPOINT_IP> <NS_MAC> <NS_FLANNEL_SUBNET> <NS_FLANNEL_SUBNET_MASK>`

`# python NSK8sConfig.py addmac 10.11.50.10 10.11.50.13 d2:15:53:cd:46:60 10.254.51.0 24`

### Step 3:
The guestbook replication controllers and services should be started now. This can be done via the consolidated spec made available:

`# kubectl create -f guestbook-allinone.yaml`

This will create one instance of redis-master, 2 instances of redis-slave and 3 instances of frontend by default. 

###### Note: The yaml files are ontainable in the tree at contrib/ingress/controllers/citrix-netscaler/example/guestbook

All the required services for the deployment of guestbook app are complete. Even so, the app is not available to the external world at this stage. 

----

Now is a good time to examine the state of the cluster. 

    # kubectl get nodes
    NAME           STATUS    AGE
    kube-minion1   Ready     16d
    kube-minion2   Ready     16d
    
    # kubectl get rc
    NAME           DESIRED   CURRENT   AGE
    frontend       3         3         12h
    redis-master   1         1         12h
    redis-slave    2         2         12h
    
    # kubectl get pods
    NAME                 READY     STATUS    RESTARTS   AGE
    frontend-75j9f       1/1       Running   0          12h
    frontend-j6z2b       1/1       Running   0          12h
    frontend-xrae0       1/1       Running   0          11h
    redis-master-ztaci   1/1       Running   0          12h
    redis-slave-14lit    1/1       Running   0          12h
    redis-slave-lne1u    1/1       Running   0          12h
    
    # kubectl get services
    NAME           CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
    frontend       10.254.212.187   <none>        80/TCP     12h
    kubernetes     10.254.0.1       <none>        443/TCP    16d
    redis-master   10.254.55.39     <none>        6379/TCP   12h
    redis-slave    10.254.12.131    <none>        6379/TCP   12h

----

### Step 4:
Next step is to configure the NetScaler credentials using Kubernetes secret. The secured data will be made available to the pod utilizing the secret via environment variables.

The secret required is username and password for NetScaler that should be encoded in base64 and provided in yaml file.

To encode to base64 the following can be done: 
`# echo -n <credential> | base64`

To decode from base64 the following can be done: 
`# echo <encoded-credential> | base64 -d`

`# kubectl create -f NS-login-secret.yaml`

### Step 5:
The next steps is to deploy the ingress controller. This controller is the custom controller provided by Citrix. It will be listening for addition and removal of ingresses as well as any changes to the endpoints associated with services for which ingresses are created. The ingress controller runs as a pod within the cluster. 

Before the deployment of ingress controller, the spec file associated with it should be modified. Specifically the NetScaler management address must be provided withing the spec. There are two ways in which the ingress controller can talk to Kubernetes API server. One is via the default service account. In case this is not available or configured, environment variables for API server address and port can be configured. 

`# kubectl create -f NS-ingress-controller.yaml`

### Step 6:
As a final step we would create the ingress for the frontend service. The VIP for the service on NetScaler should be configured as an annotation in this spec. In addition the hostname association for content switching should also be provided as part of this spec.

    # cat frontend-ingress.yaml
    apiVersion: extensions/v1beta1
    kind: Ingress
    metadata:
    name: ingress-frontend
    annotations:
        publicIP: "10.217.129.70"
        port: "80"
        protocol: HTTP
    spec:
    rules:
    - host: k8s.citrix.com
        http:
        paths:
        - backend:
            serviceName: frontend
            servicePort: 80

`# kubectl create -f frontend-ingress.yaml`

The ingress controller will detect the presence of a new ingress and configure NetScaler to act as a content switching load balancer for the incoming traffic. Now the frontend, which was previously inaccessible, will become reachable via the NetScaler to the external world.

To access the guestbook using curl perform this:

`# curl -s -i -k -H "Host:k8s.citrix.com" -X GET http://10.217.129.70/`

To access the guesbook using browser, modify the request header with value of Host set to k8s.citrix.com and use url:

`http://10.217.129.70/`

The host header is being set here to k8s.citrix.com because of the rule provided in the ingress spec that only forwards the request to the public IP specified in case the host value matches. 

----

The final state of the cluster is as follows:

    # kubectl get nodes
    NAME           STATUS    AGE
    kube-minion1   Ready     16d
    kube-minion2   Ready     16d
    
    # kubectl get rc
    NAME           DESIRED   CURRENT   AGE
    frontend       3         3         12h
    nsingress      1         1         7h
    redis-master   1         1         12h
    redis-slave    2         2         12h
    
    # kubectl get pods
    NAME                 READY     STATUS    RESTARTS   AGE
    frontend-75j9f       1/1       Running   0          12h
    frontend-j6z2b       1/1       Running   0          12h
    frontend-xrae0       1/1       Running   0          12h
    nsingress-obqn0      1/1       Running   0          7h
    redis-master-ztaci   1/1       Running   0          12h
    redis-slave-14lit    1/1       Running   0          12h
    redis-slave-lne1u    1/1       Running   0          12h
    
    # kubectl get services
    NAME           CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
    frontend       10.254.212.187   <none>        80/TCP     12h
    kubernetes     10.254.0.1       <none>        443/TCP    16d
    redis-master   10.254.55.39     <none>        6379/TCP   12h
    redis-slave    10.254.12.131    <none>        6379/TCP   12h
    
    # kubectl get secrets
    NAME              TYPE      DATA      AGE
    ns-login-secret   Opaque    2         12h
    
    # kubectl get ingress
    NAME               RULE             BACKEND   ADDRESS   AGE
    ingress-frontend   -                                    7h
                    k8s.citrix.com
                                        frontend:80

In addition to the configuration on the Kubernetes cluster, the following commands can be used to observe the configuration performed on NetScaler via ingress controller:
- show ns ip 
- show cs vserver
- show cs policy 
- show cs action
- show lb vserver
- show services

----

Appendix 1: Theory of operation
-----------
A typical Kubernetes cluster deployment consists of one Kube-master node and multiple Kube-worker nodes. A NetScaler will reside outside of the cluster and direct traffic towards it. The NetScaler works in conjunction with an ingress controller that provides configuration updates and manages the NetScaler based on the events happening inside the Kubernetes cluster.  

The current implementation of ingress controller for NetScaler provides the following functionality: 
- It looks for additions and removals of ingresses in Kubernetes cluster.
- It looks for additions and removals of endpoints associated with services manages by ingresses. 
- It dynamically configures NetScaler configuration based on changes to ingresses and endpoints. 
- It identifies the current state of the cluster upon startup. 

The ingress controller identifies the endpoints associated with the service that the ingress is supporting. Once identified, the NetScaler instance is directed to load balance directly between the endpoint addresses bypassing the service Cluster-IP associated with the service.

The communication between the NetScaler and worker nodes happens over VXLAN. The NetScaler credentials are provided using Kubernetes secrets. 

The following actions are taken upon the detection of a new ingress by the ingress controller on NetScaler:
- Identifies the public IP address/VIP and port associated with ingress as part of annotations.
- Identifies the host associated with each ingress rule. 
- Identifies service associated with each rule. 
- Identifies the endpoints serving the service. 
- Creates a content switching virtual server.
- Creates a NetScaler service for each endpoint.
- Creates a LB virtual server to front the service.
- Binds the LB to the service.
- Creates a content switching action to switch to the LB.
- Creates a content switching policy to use the action.
- Binds the content switching policy to the content switching virtual server.

On identifying that a previously seen ingress is no longer present, the above actions are undone on the NetScaler VPX instance. 

The following actions are taken upon the detection of a new endpoint by the ingress controller on Netscaler: 
- It creates a new service for the endpoint.
- It binds the service with the existing load balancer associated with other services of the same type.

On identifying that a previously seen endpoint is no longer present, the service created for the endpoint is removed from the NetScaler VPX instance. 

----

Appendix 2: Create container of ingress controller
-----------
The ingress controller once compiled needs to be deployed as a container within the Kubernetes cluster. The following steps can be used for creating this container using the Dockerfile made available. The container will contain the compiled ingress controller with the default action of the container being to run the ingress controller. 

`# docker build -t <containername:tag> .`

The container should be made available at a location where it can be retrieved at the time of its usage. Dockerhub was used for our purposes.

----

Appendix 3: Scaling pods
-----------
The scaling of pods/endpoints associated with service managed by ingress controller is taken care of by the ingress controller. In case of scaling up, the netscaler services  are increased to match the number of pods and on scale down again the number of pods and services are matched. 

The scaling can be performed like this:

`# kubectl scale rc frontend --replicas=4`