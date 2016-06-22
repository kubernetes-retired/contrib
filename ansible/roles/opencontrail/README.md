OpenContrail playbook role
==========================

This playbook installs opencontrail on k8s/openshift masters and nodes. It also (optionally)
installs a software gateway that provides connectivity between the underlay and the Public network
as well as connectivity between the internal underlay network where masters and minions are present
and a system network such as kube-system/default.

## User settings

### Per-host settings

Variables that can be set on a per host basis on the inventory file. Example:
```
[nodes]
kube-contrail-node-01 opencontrail_interface=eth1
```

Variable  | Description | Default 
----------|-------------|---------
opencontrail_interface | Physical interface used by the vrouter kernel module | eth0
opencontrail_ipaddr | IP address prefix | address of vhost0 when present, or the address of opencontrail_interface
opencontrail_gateway | IP address | default router on vhost0 interface

### Global settings

Variables set in inventory file.

The inventory file should define a variable group such as:
```
[opencontrail:children]
masters
nodes
gateways

[opencontrail:vars]
opencontrail_public_subnet = 192.168.254.0/24
[...]
```

| Variable | Description | Default value |
|----------|-------------|---------------|
| opencontrail_public_subnet | IP subnet of the Public network | (mandatory) |
| opencontrail_http_proxy | Proxy used by kmod builder | optional |
| opencontrail_dns_forwarder| DNS forwarder | optional |
| opencontrail_use_systemd | TODO: Use systemd to start docker containers | true |
| opencontrail_release | TODO: Software release to install | 2.20 |      

## Playbook

The objective of this playbook is that the opencontrail (and opencontrail_provision) roles should be usable when called from the kubernetes playbook, openshift or a standalone playbook such as:

```
- hosts: all
  sudo: yes
  roles: opencontrail
  vars:
    opencontrail_cluster_type: x
    # ... other vars ...
```

The *opencontrail* role has the assumption that docker is installed and running in the hosts.
The *opencontrail_provision* role has the assumption that both the kubernetes apiserver and the contrail-api are running and accepting requests.

## Variables used by the opencontrail playbook

The interface configuration facts can be established by:
 - explicit configuration in the inventory;
 - examining the physical interface (before the vhost0 interface is configured);
 - examining the vhost0 interface (after the vhost0 interface is configured)

|Variable | Description |
|---------| ---- |
| opencontrail_cluster_type | {kubernetes, openshift} |
| opencontrail_host_interface | physical interface for vrouter |
| opencontrail_host_ipaddr | IP address prefix for the vhost0 interface |
| opencontrail_host_address | IP address of the vhost0 interface |
| opencontrail_host_netmask | IP netmask of the vhost0 interface |
| opencontrail_host_gateway | Default router, when the default route is through vhost0 |

The following are determined from the variables passed into the role by either ansible_facts or the playbook predecessor tasks.

|Variable | Description |
|---------| ---- |
| opencontrail_host_kernel_tag | Kernel version |
| opencontrail_all_service_addresses | ClusterIP range |
| opencontrail_all_release | |
| opencontrail_master_ifmap_port | 8444 (openshift) |

## Pre-requisites

### kubernetes
    - ansible_facts

### openshift
    - openshift_facts
