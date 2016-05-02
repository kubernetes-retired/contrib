# Zookeeper

This example runs zookeeper through a petset.

## Bootstrap

Create the petset in this directory
```
$ kubetl create -f zookeeper.yaml
```

This example requires manual intervention. Starting up all zookeeper nodes at
once leads to an issue because we initialize the first one differently. "First" is
not defined based on index to allowe custom naming, it is instead based on the size
of the peer list.

Once you have all 3 nodes in Running, you can run the "test.sh" script in this directory.

Note that there are actually 2 ways to manage zookeeper, static and dynamic.
Static -> use the on-change script
Dyanmic -> use the on-start script

## Failover
## Scaling
### Up
### Down
## Image Upgrade
## Maintenance
## Caveats
