This filesystem branch contains a very simple example CNI plugin.
This plugin invokes `docker network` commands to connect or disconnect
a container.  To meet all of the functional requirements of
http://kubernetes.io/docs/admin/networking/#kubernetes-model some
additional static configuration is required, which you must do
yourself.
