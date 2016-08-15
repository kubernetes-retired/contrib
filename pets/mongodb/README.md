# MongoDB

This example runs mongodb through a petset.

## Bootstrap

Create the petset:
```
$ kubectl create -f mongodb.yaml
```

Once you have all 3 nodes in running, you can run the "test.sh" script in this directory, which will insert a key into the primary and check the secondaries for output.

## Failover

One can check the roles being played by each node by using the following:
```console
$ kubectl exec mongodb-0 -- /usr/bin/mongo --eval="printjson(rs.isMaster())"

MongoDB shell version: 3.2.8
connecting to: test
{
	"hosts" : [
		"mongodb-0.mongodb.default.svc.cluster.local:27017",
		"mongodb-1.mongodb.default.svc.cluster.local:27017",
		"mongodb-2.mongodb.default.svc.cluster.local:27017"
	],
	"setName" : "rs0",
	"setVersion" : 3,
	"ismaster" : true,
	"secondary" : false,
	"primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
	"me" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
	"electionId" : ObjectId("7fffffff0000000000000001"),
	"maxBsonObjectSize" : 16777216,
	"maxMessageSizeBytes" : 48000000,
	"maxWriteBatchSize" : 1000,
	"localTime" : ISODate("2016-08-15T08:43:47.598Z"),
	"maxWireVersion" : 4,
	"minWireVersion" : 0,
	"ok" : 1
}


```
This lets us see which member is primary.

Let us now test persistence and failover. First, we insert a key:
```console
$ kubectl exec mongodb-0 -- /usr/bin/mongo --eval="printjson(db.test.insert({key1: 'value1'}))"

MongoDB shell version: 3.2.8
connecting to: test
{ "nInserted" : 1 }
```

Watch existing members:
```console
$ kubectl run --attach bbox --image=mongo:3.2 --restart=Never -- sh -c 'while true; do for i in 0 1 2; do echo mongodb-$i $(mongo --host=mongodb-$i.mongodb --eval="printjson(rs.isMaster())" | grep primary); sleep 1; done; done';

Waiting for pod default/bbox to be running, status is Pending, pod ready: false
mongodb-1 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-2 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-0 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-1 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-2 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-0 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-1 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",

```

Kill the primary and watch as a new master getting elected.
```console
$ kubectl delete pod mongodb-0

pod "mongodb-0" deleted
```

Delete all pods and let the petset controller bring it up.
```console
$ kubectl delete po -l app=mongodb
$ kubectl get po --watch-only
NAME        READY     STATUS        RESTARTS   AGE
mongodb-0   0/1       Pending   0         0s
mongodb-0   0/1       Pending   0         0s
mongodb-0   0/1       Init:0/2   0         0s
mongodb-0   0/1       Init:1/2   0         10s
mongodb-0   0/1       Init:1/2   0         11s
mongodb-0   0/1       PodInitializing   0         15s
mongodb-0   0/1       Running   0         16s
mongodb-0   1/1       Running   0         20s
mongodb-1   0/1       Pending   0         0s
mongodb-1   0/1       Pending   0         0s
mongodb-1   0/1       Init:0/2   0         0s
mongodb-1   0/1       Init:1/2   0         11s
mongodb-1   0/1       Init:1/2   0         12s
mongodb-1   0/1       PodInitializing   0         15s
mongodb-1   0/1       Running   0         16s
mongodb-1   1/1       Running   0         20s
mongodb-2   0/1       Pending   0         0s
mongodb-2   0/1       Pending   0         0s
mongodb-2   0/1       Init:0/2   0         0s
mongodb-2   0/1       Init:1/2   0          19s
mongodb-2   0/1       Init:1/2   0         20s
mongodb-2   0/1       PodInitializing   0         23s
mongodb-2   0/1       Running   0         24s
mongodb-2   1/1       Running   0         31s

...

mongodb-0 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-1 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
mongodb-2 "primary" : "mongodb-0.mongodb.default.svc.cluster.local:27017",
```

Check the previously inserted key:
```console
$ kubectl exec mongodb-1 -- /usr/bin/mongo --eval="rs.slaveOk(); db.test.find({key1:{\$exists:true}}).forEach(printjson)"

MongoDB shell version: 3.2.8
connecting to: test
{ "_id" : ObjectId("57b180b1a7311d08f2bfb617"), "key1" : "value1" }
```

## Scaling

You can scale up by modifying the number of replicas on the PetSet.

## Image Upgrade

TODO: Add details

## Maintenance

TODO: Add details

## Limitations
* Both petset and init containers are in alpha
* Look through the on-start script for TODOs
* Doesn't support the addition of observers through the petset
* Only supports storage options that have backends for persistent volume claims


# TODO
* Set up authorization between replicaset peers.
* Set up sharding.