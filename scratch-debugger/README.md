# Scratch Debugger

This is a tool to make debugging containers based on scratch easier. The script
works by bringing up a pod with a statically-linked busybox image on the same
node as the debug target, mounting the node's root filesystem, and calling
docker directly to copy busybox into the target container. Once the "install" is
complete, the target can be debugged through a standard kubectl exec.

## Usage

```
scratch-debugger/debug.sh POD_NAME [POD_NAMESPACE CONTAINER_NAME]
```

- `POD_NAME` - The name of the pod to debug.
- `POD_NAMESPACE` - The namespace of the target pod (defaults to `default`).
- `CONTAINER_NAME` - The name of the container in the pod to debug (defaults to the first container).

Additionally, the following environment variables can be set:

- `TMP_SUBDIR` - The subdirectory under `/tmp` to install busybox into (defaults to `debug-tools`).
- `KUBECONTEXT` - The kubectl context to use (defaults to current context).
- `DEBUGGER_NAME` - The name to use for the debug pod (defaults to `debugger`).
- `ARCH` - The architecture Kubernetes is running on (defaults to `amd64`).

## Example

Create a simple `pause` pod, which is based off a scratch image and does nothing.
```
$ kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name:   pause
spec:
  containers:
    - name:    pause
      image: gcr.io/google_containers/pause
EOF

pod "pause" created
```

Note that we cannot simply exec into the pod, since there isn't a shell or any
other interactive tools available:
```
$ kubectl exec -i -t pause -- sh
rpc error: code = 2 desc = "oci runtime error: exec failed: exec: \"sh\": executable file not found in $PATH"
```

So we use the `debug.sh` script to copy busybox (which includes many common
tools) into the container:
```
$ scratch-debugger/debug.sh pause
Debug Target Container:
  Pod:          pause
  Namespace:    default
  Node:         e2e-test-stclair-minion-group-phj6
  Container:    pause
  Container ID: 80b134ab6550d34684cdb31e4300ff128f9f43f67fdb3d271372f9417e546737
  Runtime:      docker

Installing busybox to /tmp/debug-tools ...
pod "debugger" created
waiting for debugger pod to become ready...
Installation complete.
To debug pause, run:
    kubectl exec -i -t pause -- /tmp/debug-tools/sh -c 'PATH=$PATH:/tmp/debug-tools sh'
Dumping you into the pod container now.

/ # ls
dev    etc    pause  proc   sys    tmp    var
/ # echo Hello world!
Hello world!
/ # exit
pod "debugger" deleted
```

The script automatically execs into the pod and starts a shell (`ash`) with the
`PATH` variable set to include the debug tools. After exiting, the tools are
still present in the pod, and we can simply exec back in using the command the
script gave us:

```
$ kubectl exec -i -t pause -- /tmp/debug-tools/sh -c 'PATH=$PATH:/tmp/debug-tools sh'
/ # which sh
/tmp/debug-tools/sh
/ # exit
```

Alternatively, we can just call the `debug.sh` script again:
```
$ scratch-debugger/debug.sh pause
Debug tools already installed. Dumping you into the pod container now.
/ # exit
```

Once we've finished debugging, it's a good practice to delete the "tainted"
pod. If that is undesirable for some reason, you can simply delete the tools
from the container:
```
$ kubectl exec pause -- /tmp/debug-tools/rm -r /tmp/debug-tools
```
