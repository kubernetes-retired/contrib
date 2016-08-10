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
You can test the scenario by killing process of one of the pets:

```shell
$ ps aux | grep etcd-1
$ kill -9 ETCD_1_PID
```

```shell
$ kubectl get pods -l "app=etcd"
NAME                 READY     STATUS        RESTARTS   AGE
etcd-0               1/1       Running       0          54s
etcd-2               1/1       Running       0          51s
```

After a while:

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
1. run ``hack/local-cluster-up.sh`` with both ``ENABLE_HOSTPATH_PROVISIONER`` and ``KUBE_ENABLE_CLUSTER_DNS`` to ``true``:

   ```shell
   KUBE_ENABLE_CLUSTER_DNS=true ENABLE_HOSTPATH_PROVISIONER=true ./hack/local-up-cluster.sh
   ```

1. run skydns
1. create the petset

## Scaling

The etcd cluster can be scale up by running ``kubectl patch`` or ``kubectl edit``. For instance,

```sh
$ kubectl get pods -l "app=etcd"
NAME      READY     STATUS    RESTARTS   AGE
etcd-0    1/1       Running   0          7m
etcd-1    1/1       Running   0          7m
etcd-2    1/1       Running   0          6m

$ kubectl patch petset/etcd -p '{"spec":{"replicas": 5}}'
"etcd" patched

$ kubectl get pods -l "app=etcd"
NAME      READY     STATUS    RESTARTS   AGE
etcd-0    1/1       Running   0          8m
etcd-1    1/1       Running   0          8m
etcd-2    1/1       Running   0          8m
etcd-3    1/1       Running   0          4s
etcd-4    1/1       Running   0          1s
```

Scaling-down is similar. For instance, changing the number of pets to ``4``:

```sh
$ kubectl edit petset/etcd
petset "etcd" edited

$ kubectl get pods -l "app=etcd"
NAME      READY     STATUS    RESTARTS   AGE
etcd-0    1/1       Running   0          8m
etcd-1    1/1       Running   0          8m
etcd-2    1/1       Running   0          8m
etcd-3    1/1       Running   0          4s
```

Once a pet is terminated (either by running ``kubectl delete pod etcd-ID`` or scaling down),
content of ``/var/run/etcd/`` directory is cleaned up.
If any of the etcd pets restarts (e.g. caused by etcd failure or any other),
the directory is kept untouched so the pet can recover from the failure.

