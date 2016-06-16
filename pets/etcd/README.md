# Etcd

This example runs etcd through a petset.

## Bootstrap

Create the petset specified in ``etcd-petset.yaml``:

```shell
$ kubectl create -f etcd-petset.yaml
service "etcd" created
petset "etcd" created
```

The petset controller creates an etcd cluster of size 3.
You can check all pets are running by

```shell
$ kubectl get pods -l "app=etcd"
NAME      READY     STATUS    RESTARTS   AGE
etcd-0    1/1       Running   0          3m
etcd-1    1/1       Running   0          2m
etcd-2    1/1       Running   0          3m
```

Listing all members from inside a pet:

```shell
$ etcdctl member list
2e80f96756a54ca9: name=etcd-0 peerURLs=http://etcd-0.etcd:2380 clientURLs=http://etcd-0.etcd:2379
7fd61f3f79d97779: name=etcd-1 peerURLs=http://etcd-1.etcd:2380 clientURLs=http://etcd-1.etcd:2379
b429c86e3cd4e077: name=etcd-2 peerURLs=http://etcd-2.etcd:2380 clientURLs=http://etcd-2.etcd:2379
```

## Failover

If any etcd member fails it gets re-joined eventually.
You can test the scenario by deleting one of the pets:

```shell
$ kubectl delete pod etcd-1
```

```shell
$ kubectl get pods -l "app=etcd"
NAME                 READY     STATUS        RESTARTS   AGE
etcd-0               1/1       Running       0          54s
etcd-1               1/1       Terminating   0          52s
etcd-2               1/1       Running       0          51s
```

```shell
$ kubectl get pods -l "app=etcd"
NAME                 READY     STATUS    RESTARTS   AGE
etcd-0               1/1       Running   0          1m
etcd-1               1/1       Running   0          20s
etcd-2               1/1       Running   0          1m
```

You can check state of re-joining from ``etcd-1``'s logs:

```shell
$ kubectl logs etcd-1
Waiting for etcd-0.etcd to come up
Waiting for etcd-1.etcd to come up
ping: bad address 'etcd-1.etcd'
Waiting for etcd-1.etcd to come up
Waiting for etcd-2.etcd to come up
Re-joining etcd member
Updated member with ID 7fd61f3f79d97779 in cluster
2016-06-20 11:04:14.962169 I | etcdmain: etcd Version: 2.2.5
2016-06-20 11:04:14.962287 I | etcdmain: Git SHA: bc9ddf2
...
```

## Testing

1. provision one node cluster
1. build HEAD of master branch
1. run ``hack/local-cluster-up.sh`` with ``ENABLE_HOSTPATH_PROVISIONER`` set to ``true``:

   ```shell
   ENABLE_HOSTPATH_PROVISIONER=true ./hack/local-up-cluster.sh
   ```

1. run skydns
1. create the petset

## Scaling

TODO

## Limitations

* no scaling
