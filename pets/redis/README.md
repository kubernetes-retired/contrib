# Redis

This example runs redis through a petset.

## Master/slave

### Bootstrap

Create the yaml in this directory
```
$ kubectl create -f redis.yaml
```

can run the "test.sh" script in this directory.

## TODO

Expect cleaner solutions for the following as petset matures.

* Scaling Up/down
* Image Upgrade
* Periodic maintenance
* Sentinel failover
