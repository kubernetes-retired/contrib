# Mysql Galera

This example runs mysql galera through a petset.

## Bootstrap

Create the petset in this directory
```
$ kubectl create -f galera.yaml
```

This example requires manual intervention. Starting up all galera nodes at once
leads to an issue where all the mysqls belive they're in the primary component
because they don't see the others in the DNS. For the bootstrapping to work:
mysql-0 needs to see itself, mysql-1 needs to see itself and mysql-0, and so on,
because the first node that sees a peer list of 1 will assume it's the leader.

Once you have all 3 nodes in Running, you can run the "test.sh" script in this directory.

## Failover
## Scaling
### Up
### Down
## Image Upgrade
## Maintenance
## Caveats
