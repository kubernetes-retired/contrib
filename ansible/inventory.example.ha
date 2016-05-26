# TODO: Current multi-master configuration doesn't allow for
# a single host to be both a master and node.
# https://github.com/kubernetes/contrib/issues/838

[masters]
kube-master-test-[1:3].example.com

[etcd:children]
masters

[nodes]
kube-minion-test-[1:2].example.com
