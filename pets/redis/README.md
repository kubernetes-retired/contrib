# Redis

This example runs redis through a petset.

## Master/slave

### Bootstrap

Create the yaml in this directory
```
$ kubectl create -f redis.yaml
```

Wait till the first redis pod becomes master:
```
$ kubectl exec rd-0 redis-cli -h rd-0.redis info | grep role
```

Annotate it to ready.
```
$ kubectl annotate pod petset_name-0 --overwrite pod.alpha.kubernetes.io/initialized="true"
```
Let the other nodes start as slaves. Once you have all 3 nodes in Running, you
can run the "test.sh" script in this directory.

## Scaling
### Up
### Down
## Image Upgrade
## Maintenance


## Sentinel

### Failover

## Caveats
