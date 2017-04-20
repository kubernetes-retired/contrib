# Kubernetes-Demo
N-Tier Applications/Services deployed on Kubernetes
three-card-poker game with data, atomic, composite and presentation layer to demonstrate kubernetes ecosystem


![N Tier picture](https://github.com/suyogbarve/kubernetes-demo/blob/master/NTierServices.jpg?raw)

In this demo we will create a three-card-poker application using N-Tier micro-service architecture which 
will comprise of a data layer, atomic services, composite services and presentation layer.
We will also deploy a demo-client application which continuously polls the master-api to show 
number of pods in each layer mentioned above. 

Steps

0. Deploying demo-client-app: This app provides a dynamic view for each layer/pods described below. It polls master api to reflect each pods
in layers mentioned below. This provides a very good picture of how quickly pods are created/scaled in kubernetes cluster
It requires an env variable MASTER_SERVICE which points towards IP address of Kubernetes master
demo-client-app-deployment.yaml and demo-client-app-service.yaml can be used to create it
Test it here: http://MASTER:8080/api/v1/proxy/namespaces/default/services/demo-client-app/demo/#/default
Note: replace last "default" with namespace where layers 1,2,3,4,5 are running (if you choose to run them in a different namespace)

1. Deploying redis database.
redis-deployment.yaml and redis-service.yaml are used to create deployment and service for redis pods
2. Deploying redis-a atomic app: This represents an atomic service which simply returns a random card/number between 1 to 13 (Ace,2,3...10, Jack,Queen,King)
redis-a-app-deployment.yaml and redis-a-app-service.yaml are used to create service for redis-a app
Test it here: http://MASTER:8080/api/v1/proxy/namespaces/default/services/redis-a-app/randomNumber

3. Deploying redis-b atomic app: This represents an atomic service which simply returns a random suit (diamond,heart,spade,clubs)
redis-b-app-deployment.yaml and redis-b-app-service.yaml are used to create service for redis-a app
Test it here: http://MASTER:8080/api/v1/proxy/namespaces/default/services/redis-b-app/randomSuit

4. Deploying redis-composite app: This app represents a composite service which uses underlying atomic services (redis-a and redis-b)
redis-composite-app-deployment.yaml and redis-composite-app-service.yaml are used to deploy this composite layer
Test it here: http://MASTER:8080/api/v1/proxy/namespaces/default/services/redis-composite-app/card

5. Deploying three-card-poker app: This app consumes the composite layer to create a final presentation layer
a. three-card-poker : It is a php based 
b. three-card-poker-java : It is a java (spring boot) based app
They can be deployed using three-card-poker-deployment.yaml and three-card-poker-service.yaml
Test it here : MASTER:8080/api/v1/proxy/namespaces/default/services/three-card-poker/index.html



Extra Info
For simplicity, Kubernetes Master based Test URLs are used to access each service, however one should use NodePort or Loadbalancer approach to 
access those services.
heapster-svc.yaml and  heapster-rc.yaml can be used to deploy heapster in the cluster; heapster service is required to perform autoscaling
For each deployment above autoscaling can be used if resource limit is defined in deployment yaml file (example in three-card-poker-deployment.yaml) 
following kubectl commands can be used to for autoscaling
kubectl autoscale deployment three-card-poker --min=1 --max=10 --cpu-percent=50
kubectle get hpa
Note: It is important to use correct version of heapster depending on your kubernetes version. Like kubernetes 1.3.5 supports heapster:v1.1.0-beta2


kube-dash.yaml can be used to deploy kubernetes dashboard
kube-dns-rc.yaml and kube-dns-svc.yaml can be used to deploy DNS service in cluster (although DNS service is not required for this demo)



![demo-client-app](https://github.com/suyogbarve/kubernetes-demo/blob/master/demo-client-app.png?raw)
![three-card-poker](https://github.com/suyogbarve/kubernetes-demo/blob/master/three-card-poker.png?raw)
Note the small friendly bug image on three-card-poker app, this can be used as a usecase to show rolling updates once the bugfix is done.

Finally, remember to start redis-app (database), before starting redis-a and redis-b. Spring boot container would take 60-70 seconds to start.




