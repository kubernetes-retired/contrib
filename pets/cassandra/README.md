# Cassandra

This example runs Cassandra through a petset.

## Bootstrap

Create the petset in this directory
```
$ kubectl create -f cassandra.yaml
```

This example requires manual intervention. Starting up all cassandra nodes at once leads to an issue:
```
INFO  18:46:57 Handshaking version with /10.245.2.6
INFO  18:46:57 InetAddress /10.245.2.6 is now UP
Exception (java.lang.UnsupportedOperationException) encountered during startup: Other bootstrapping/leaving/moving nodes detected, cannot bootstrap while cassandra.consistent.rangemovement is true
```

You want to wait till you see something like
```
INFO  19:07:27 Node cs-2.cassandra.default.svc.cluster.local/10.245.2.6 state jump to NORMAL
INFO  19:07:27 Waiting for gossip to settle before accepting client requests...
INFO  19:07:35 No gossip backlog; proceeding
```
in the output of kubectl logs, and then apply the annotation: `pod.alpha.kubernetes.io/initalized=true`.

Once you have all 3 nodes in Running, you can run the "test.sh" script in this directory.

## Failover
## Scaling
### Up
### Down
## Image Upgrade
## Maintenance
## Caveats
