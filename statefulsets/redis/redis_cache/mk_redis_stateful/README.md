##A Simple tool generate redis statefulset yaml file, with the given name and replica
```
$./mk_redis_statefulset -h
Usage of ./mk_redis_statefulset:
-disruptionbudge int
	Number number of guaranteed instances during K8s Maintenance activity (default 2)
-name string
	Name of the redis statefulset (default "cache")
-replicas int
	Number of redis instances you need in this master slave setup (default 3)
```

To create a redis-cache statefulset called as _'test'_ which has one master and 3 slaves
```
$./mk_redis_statefulset -name test -replicas 4 > test.yaml
```

Push this to kubernetes cluster
```
kubectl create -f test.yml
```

Check if the statefulset has been created
```
$kubectl describe statefulset test
Name:                   test
Namespace:              default
Image(s):               redis:3.0-alpine,dhilipkumars/mk-redis-slave
Selector:               app=rd-test
Labels:                 app=rd-test
Replicas:               0 current / 4 desired
Annotations:            <none>
CreationTimestamp:      Tue, 14 Feb 2017 18:49:12 +0530
Pods Status:            0 Running / 1 Waiting / 0 Succeeded / 0 Failed
Volumes:
  config:
    Type:       EmptyDir (a temporary directory that shares a pod's lifetime)
    Medium:
Events:
  FirstSeen     LastSeen        Count   From            SubObjectPath   Type            Reason                  Message
  ---------     --------        -----   ----            -------------   --------        ------                  -------
  22s           22s             1       statefulset                     Normal          SuccessfulCreate        create Pod test-0 in StatefulSet test successful
```
