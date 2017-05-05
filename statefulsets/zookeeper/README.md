#Kubernetes ZooKeeper K8SZK
This project contains a Docker image meant to facilitate the deployment of 
[Apache ZooKeeper](https://zookeeper.apache.org/) on [Kubernetes](http://kubernetes.io/) using 
[StatefulSets](http://kubernetes.io/docs/abstractions/controllers/petset/). 
##Limitations
1. Scaling is not currently supported. An ensemble's membership can not be updated in a safe way 
in ZooKeeper 3.4.9 (The current stable release).
2. Observers are currently not supported. Contributions are welcome.
3. Persistent Volumes must be used. emptyDirs will likely result in a loss of data.

##Docker Image
The docker image contained in this repository is comprised of a base Ubuntu 16.04 image using the latest
release of the OpenJDK JRE based on the 1.8 JVM (JDK 8u111) and the latest stable release of 
ZooKeeper, 3.4.9. Ubuntu is a much larger image than BusyBox or Alpine, but these images contain 
mucl or ulibc. This requires a custom version of OpenJDK to be built against a libc runtime other 
than glibc. No vendor of the ZooKeeper software supplies or verifies the software against such a 
JVM, and, while Alpine or BusyBox would provide smaller images, we have prioritized a well known 
environment.

The image is built such that the ZooKeeper process is designated to run as a non-root user. By default, 
this user is zookeeper. The ZooKeeper package is installed into the /opt/zookeeper directory, all 
configuration is sym linked into the /usr/etc/zookeeper/, and all executables are sym linked into 
/usr/bin. The ZooKeeper data directories are contained in /var/lib/zookeeper. This is identical to 
the RPM distribution that users should be familiar with.
##Configuration
###Headless Service
The ZooKeeper Stateful Set requires a Headless Service to control the network domain for the 
ZooKeeper processes. An example configuration is provided below.
```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
  name: zk-headless
  labels:
    app: zk-headless
spec:
  ports:
  - port: 2888
    name: server
  - port: 3888
    name: leader-election
  clusterIP: None
  selector:
    app: zk-headless
```
Note that the Service contains two ports. The server port is used for followers to tail the leaders
even log, and the leader-election port is used by the ensemble to perform leader election.
###Stateful Set
The Stateful Set configuration must match the Headless Service, and it must provide the number of 
replicas. In the example below we request a ZooKeeper ensemble of size 3. 
**As weighted quorums are not supported, it is imperative that an odd number of replicas be chosen.
Moreover, the number of replicas should be either 1, 3, 5, or 7. Ensembles may be scaled to larger 
membership for read fan out, but, as this will adversely impact write performance, careful thought
should be given to selecting a larger value.**
```yaml
apiVersion: apps/v1beta1
kind: StatefulSet
metadata:
  name: zk
spec:
  serviceName: zk-headless
  replicas: 3
```
###Container Configuration
The zkGenConfig.sh script will generate the ZooKeeper configuration (zoo.cfg), Log4J configuration
(log4j.properties), and JVM configuration (jvm.env). These will be written to the 
/opt/zookeeper/conf directory with correct read permissions for the zookeeper user. These files are 
generated from environment variables that are injected into the container as in the example, minimal 
configuration below.
```yaml
containers:
      - name: k8szk
        imagePullPolicy: Always
        image: gcr.io/google_samples/k8szk:v1
        ports:
        - containerPort: 2181
          name: client
        - containerPort: 2888
          name: server
        - containerPort: 3888
          name: leader-election
        env:
        - name : ZK_ENSEMBLE
          value: "zk-0;zk-1;zk-2"
        - name: ZK_CLIENT_PORT
          value: "2181"
        - name: ZK_SERVER_PORT
          value: "2888"
        - name: ZK_ELECTION_PORT
          value: "3888"
```
####Membership Configuration
|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:---------|
|ZK_ENSEMBLE|string|N/A|A colon separated list of servers in the ensemble.|
This is a mandatory configuration variable that is used to configure the membership of the 
ZooKeeper ensemble. It is also used to prevent data loss during accidental scale operations. The 
set can be computed as as follows. For all integers in the range [0,replicas), prepend the name of 
service followed by a dash to the integer. So for the Stateful Set above, the name is zk and we have
3 replicas. for the set {0,1,2} we prepend zk- giving us zk-0;zk-1;zk-2.

####Network Configuration
|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:--------|
|ZK_CLIENT_PORT|integer|2181|The port on which the server will accept client requests.|
|ZK_SERVER_PORT|integer|2888|The port on which the leader will send events to followers.|
|ZK_ELECTION_PORT|integer|3888|The port on which the ensemble performs leader election.|
|ZK_MAX_CLIENT_CNXNS|integer|60|The maximum number of concurrent client connections that a server in the ensemble will accept.|

The ZK_CLIENT_PORT, ZK_ELECTION_PORT, and ZK_SERVERS_PORT must be set to the containerPorts 
specified in the container configuration, and the ZK_SERVER_PORT and ZK_ELECTION_PORT 
must match the Headless Service configuration. However, if the default values of 
the environment variables are used for both the containerPorts and the Headless Service, the 
environment variables may be omitted from the configuration.

####ZooKeeper Time Configuration
|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:--------|
|ZK_TICK_TIME|integer|2000|The number of wall clock ms that corresponds to a Tick for the ensembles internal time.|
|ZK_INIT_LIMIT|integer|5|The number of Ticks that an ensemble member is allowed to perform leader election.|
|ZK_SYNC_LIMIT|integer|10|The number of Tick by which a follower may lag behind the ensembles leader.|

####ZooKeeper Session Configuration
|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:--------|
|ZK_MIN_SESSION_TIMEOUT|integer|2 * ZK_TICK_TIME|The minimum session timeout that the ensemble will allow a client to request.|
|ZK_MAX_SESSION_TIMEOUT|integer|20 * ZK_TICK_TIME|The maximum session timeout that the ensemble will allows a client to request.|

####Data Retention Configuration
**ZooKeeper does not, by default, purge old transactions logs or snapshots. This can cause 
the disk to become full.** If you have backup procedures and retention policies that rely on 
external systems, the snapshots can be retrieved manually from the /var/lib/zookeeper/data directory,
and the logs can be retrieved manually from the /var/lib/zookeeper/log directory.
These will be stored on the persistent volume. The zkCleanup.sh script can be used to manually purge
outdated logs and snapshots.

If you do not have an existing retention policy and backup procedure, and if you are comfortable with 
an automatic procedure, you can use the environment variables below to enable and configure 
automatic data purge policies.

|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:---------|
|ZK_SNAP_RETAIN_COUNT|integer|3|The number of snapshots that the ZooKeeper process will retain if ZK_PURGE_INTERVAL is set to a value greater than 0.|
|ZK_PURGE_INTERVAL|integer|0|The delay, in hours, between ZooKeeper log and snapshot cleanups.|

####JVM Configuration
Currently the only supported JVM configuration is the JVM heap size. Be sure that the heap size you
request does not cause the process to swap out.

|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:--------|
|ZK_HEAP_SIZE|integer|2|The JVM heap size in Gibibytes.|

####Log Level Configuration
|Variable|Type|Default|Description|
|:------:|:---:|:-----:|:--------|
|ZK_LOG_LEVEL|enum(TRACE,DEBUG,INFO,WARN,ERROR,FATAL)|INFO|The Log Level that for the ZooKeeper processes logger.|

####Liveness and Readiness
The zkOk.sh script can be used to check the liveness and readiness of ZooKeeper process. The example 
below demonstrates how to configure liveness and readiness probes for the Pods in the Stateful Set.
```yaml
  readinessProbe:
    exec:
      command:
      - sh
      - -c
      - "zkOk.sh"
    initialDelaySeconds: 15 
    timeoutSeconds: 5
  livenessProbe:
    exec:
      command:
      - sh
      - -c
      - "zkOk.sh"
    initialDelaySeconds: 15
    timeoutSeconds: 5
```
####Volume Mounts
volumeMounts for the container should be defined as below.
```yaml
  volumeMounts:
  - name: datadir
    mountPath: /var/lib/zookeeper
```
###Storage Configuration
Currently, the use of Persistent Volumes to provide durable, network attached storage is mandatory.
**If you use the provided image with emptyDirs, you will likely suffer a data loss.** The example 
below demonstrates how to request a dynamically provisioned persistent volume of 20 GiB.
```yaml
  volumeClaimTemplates:
  - metadata:
      name: datadir
      annotations:
        volume.alpha.kubernetes.io/storage-class: anything
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 20Gi
```

###Logging 
The Log Level configuration may be modified via the ZK_LOG_LEVEL environment variable as described 
above. However, the location of the log output is not modifiable. The ZooKeeper process must be 
run in the foreground, and the log information will be shipped to the stdout. This is considered 
to be a best practice for containerized applications, and it allows users to make use of the 
log rotation and retention infrastructure that already exists for K8s.

###Metrics 
The zkMetrics script can be used to retrieve metrics from the ZooKeeper process and print them to 
stdout. A recurring Kubernetes job can be used to collect these metrics and provide them to a 
collector.
```bash
bash$ kubectl exec zk-0 zkMetrics.sh
zk_version	3.4.9-1757313, built on 08/23/2016 06:50 GMT
zk_avg_latency	0
zk_max_latency	0
zk_min_latency	0
zk_packets_received	21
zk_packets_sent	20
zk_num_alive_connections	1
zk_outstanding_requests	0
zk_server_state	follower
zk_znode_count	4
zk_watch_count	0
zk_ephemerals_count	0
zk_approximate_data_size	27
zk_open_file_descriptor_count	39
zk_max_file_descriptor_count	1048576

```
