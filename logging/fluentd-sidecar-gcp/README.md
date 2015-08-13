# Collecting log files from within containers with Fluentd and sending them to the Google Cloud Logging service.
*Note that this only works for clusters running on GCE and whose VMs have the cloud-logging.write scope. If your cluster is logging to Elasticsearch instead, see [this guide](/contrib/logging/fluentd-sidecar-es/) instead.*

This directory contains the source files needed to make a Docker image that collects log files from arbitrary files within a container using [Fluentd](http://www.fluentd.org/) and sends them to GCP.
The image is designed to be used as a sidecar container as part of a pod.
It lives in the Google Container Registry under the name `gcr.io/google_containers/fluentd-sidecar-gcp`.

This shouldn't be necessary if your container writes its logs to stdout or stderr, since the Kubernetes cluster's default logging infrastructure will collect that automatically, but this is useful if your application logs to a specific file in its filesystem and can't easily be changed.

In order to make this work, you have to add a few things to your pod config:

1. A second container, using the `gcr.io/google_containers/fluentd-sidecar-gcp:1.3` image to send the logs to Google Cloud Logging. We recommend attaching resource constraints of `100m` CPU and `200Mi` memory to this container, as in the example.
2. A volume for the two containers to share. The emptyDir volume type is a good choice for this because we only want the volume to exist for the lifetime of the pod.
3. Mount paths for the volume in each container.  In your primary container, this should be the path that the applications log files are written to. In the secondary container, this can be just about anything, so we put it under /mnt/log to keep it out of the way of the rest of the filesystem.
4. The `FILES_TO_COLLECT` environment variable in the sidecar container, telling it which files to collect logs from. These paths should always be in the mounted volume.

To try it out, make sure that your cluster was set up to log to Google Cloud Logging when it was created (i.e. you set `LOGGING_DESTINATION=gcp` or are running on Container Engine), then simply run
```console
kubectl create -f logging-sidecar-pod.yaml
```

You should see the logs show up in the log viewer of the Google Developer Console shortly after creating the pod. To clean up after yourself, simply run
```console
kubectl delete -f logging-sidecar-pod.yaml
```


[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/contrib/logging/fluentd-sidecar-gcp/README.md?pixel)]()
