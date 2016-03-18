This filesystem branch contains a very simple example CNI plugin.
This plugin invokes `docker network` commands to connect or disconnect
a container.  In particular, this plugin's "add container to network"
operation connects the container to a Docker network that is
identified in the config file.

To meet all of the functional requirements of
http://kubernetes.io/docs/admin/networking/#kubernetes-model some
additional static configuration is required, which you must do
yourself.

This plugin is written in bash so that is widely understandable and
does not require any building.


## Pre-requisites

You must have Kubernetes, Docker, bash, and [jq]
(https://stedolan.github.io/jq/) installed on each Kubernetes worker
machine ("node").

Docker must be configured with a cluster store, and you must have a
multi-host Docker network.  The Docker name of that multi-host network
must appear as the value of the `name` field in `c2d.conf`.


## Installation and Configuration

This plugin has one config file and two executables.  Put the
executables in a directory of your choice; `/opt/cni/bin/` would not
be an unreasonable choice.  Put the config file, `c2d.conf`, in a
directory of your choice; `/etc/cni/net.d/` is the usual choice.

There are two configuration settings that must be made on each
kubelet.  The following expresses those settings as command line
arguments, assuming that `/etc/cni/net.d/` is the directory where you
put the config file.

```
--network-plugin=cni --network-plugin-dir=/etc/cni/net.d
```

If the config file has a field named `debug` with the value `true`
then each invocation of the plugin will produce some debugging output
in `/tmp/`.

A multi-host docker network does not necessarily meet the requirement
(seen in http://kubernetes.io/docs/admin/networking/#kubernetes-model)
that hosts can open connections to containers.  However, you can
typically enable this with a bit of static configuration.  The
particulars of this depend on the Docker network you choose; this
simple plugin does not attempt to discern and effect that static
configuration --- it is up to you.
