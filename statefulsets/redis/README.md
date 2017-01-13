## Redis-cache

Install Redis Petset as an in-memory cache. 
* Uses 7MB [redis:3-alpine](https://hub.docker.com/r/library/redis/tags/3-alpine/) image which is extreemly light weight
* Uses [redis-sentinel-micro](https://github.com/dhilipkumars/redis-sentinel-micro) for automatic slave promotion if master fails
* Super fast caching as no persistance involved

Quick Benchmark comparision between [redis petset](https://github.com/kubernetes/contrib/tree/master/pets/redis) with persistence enabled and [this redis statefulset](https://github.com/kubernetes/contrib/tree/master/statefulsets/redis), as you can notice the writes are atleast 3 times slower.

Redis Petset with persistance enabled
```
$ k exec rediscli -- redis-benchmark -q -h rd-0.redis.default.svc.cluster.local -p 6379 -t set,get -n 100000 -d 100 -r 1000000
SET: 22026.43 requests per second
GET: 76161.46 requests per second
```
Redis Statefulset configured without persistance
```
$ k exec rediscli -- redis-benchmark -q -h cache-0.cache.default.svc.cluster.local -p 6379 -t set,get -n 100000 -d 100 -r 1000000
SET: 60060.06 requests per second
GET: 89285.71 requests per second
```

Here is a sample demo of master slave promotion.

Install the Statefulset 
```
$k create -f pets/statefulsets/redis.yml
service "cache" created
petset "cache" created
```

Available service 
```
$ kd service cache
Name:                   cache
Namespace:              default
Labels:                 app=rd-cache
Selector:               app=rd-redis
Type:                   ClusterIP
IP:                     None
Port:                   peer    6379/TCP
Endpoints:              172.17.0.3:6379,172.17.0.4:6379,172.17.0.7:6379
Session Affinity:       None
No events.
```

Available pods
```
$ kp
NAME       READY     STATUS    RESTARTS   AGE
cache-0    2/2       Running   0          29m
cache-1    2/2       Running   0          28m
cache-2    2/2       Running   0          2m
rediscli   1/1       Running   0          1h
```

Index '0' is the master
```
$ k exec rediscli -- redis-cli -h cache-0.cache.default.svc.cluster.local -p 6379 info replication
# Replication
role:master
connected_slaves:2
slave0:ip=172.17.0.7,port=6379,state=online,offset=2157,lag=1
slave1:ip=172.17.0.4,port=6379,state=online,offset=2157,lag=1
master_repl_offset:2157
repl_backlog_active:1
repl_backlog_size:1048576
repl_backlog_first_byte_offset:2
repl_backlog_histlen:2156
```
lets set a key/value pair
```
$ k exec rediscli -- redis-cli -h cache-0.cache.default.svc.cluster.local -p 6379 set foo bar
OK
```
lets check if that is replicated in one of the slave,
```
$ k exec rediscli -- redis-cli -h cache-1.cache.default.svc.cluster.local -p 6379 get foo
bar
```
lets kill the master, and wait until it is re-started by the petset controller.
```
$ k delete pod cache-0
pod "cache-0" deleted

$ kp
NAME       READY     STATUS     RESTARTS   AGE
cache-0    0/2       Init:0/1   0          1m
cache-1    2/2       Running    0          37m
cache-2    2/2       Running    0          10m
rediscli   1/1       Running    0          1h


$ kp
NAME       READY     STATUS            RESTARTS   AGE
cache-0    0/2       PodInitializing   0          1m
cache-1    2/2       Running           0          37m
cache-2    2/2       Running           0          11m
rediscli   1/1       Running           0          1h
```
lets check the log for sentinel micro, in the newly created container.
```
$ kl cache-0 sentinel-micro
.
.
.
I0110 20:19:54.311197      10 redis_sentinel_micro.go:166] Processing cache-0.cache.default.svc.cluster.local
I0110 20:19:54.311213      10 redis_sentinel_micro.go:166] Processing cache-1.cache.default.svc.cluster.local
I0110 20:19:54.311221      10 redis_sentinel_micro.go:166] Processing cache-2.cache.default.svc.cluster.local
.
.

I0110 20:19:54.319381      10 redis_sentinel_micro.go:325] OldMaster=<nil> NewMaster=&{cache-2.cache.default.svc.cluster.local:6379 slave -1 9 3009 cache-0.cache.default.svc.cluster.local 6379 100 false 0xc820090180}
I0110 20:19:54.324751      10 redis_sentinel_micro.go:341] New Master is cache-2.cache.default.svc.cluster.local:6379, All the slaves are re-configured to replicate from this
I0110 20:19:54.325414      10 redis_sentinel_micro.go:364] Redis-Sentinal-micro Finished
2017/01/10 20:19:54 Peer finder exiting
```
the slave cache-2 is being promoted as the new master.
```
$ k exec rediscli -- redis-cli -h cache-0.cache.default.svc.cluster.local -p 6379 info replication
# Replication
role:slave
master_host:cache-2.cache.default.svc.cluster.local
master_port:6379
master_link_status:up
master_last_io_seconds_ago:9
.
.
```


lets check if the new slave (which was the old master) still got the 'foo bar' key replicated.
```
$k exec rediscli -- redis-cli -h cache-0.cache.default.svc.cluster.local -p 6379 get foo
bar
```
