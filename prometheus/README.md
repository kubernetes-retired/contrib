# Prometheus in Kubernetes

There are two maintained ways to run Prometheus in Kubernetes:

* [The Prometheus Operator](#the-prometheus-operator)
* [The Prometheus Helm Chart](#the-prometheus-helm-chart)

## Prometheus basics

Prometheus installations in kubernetes need to be able to discover and poll nodes, while also
ensuring that the master itself doesn't go down due to large metric volumes.

Additionally, prometheus needs to find services it cares about, and this is normally done via injection of 
a config map.

Both the prometheus operator and the prometheus helm chart solve these problems by allowing
users to declaratively map services into the prometheus configuration (operator) / allowing users
to specify a configmap at installation time (i.e. the `server.configMapOverrideName` parameter, in 
the prometheus helm chart).

To manually setup prometheus, without the use of either the operator or helm chart, you can
directly run prometheus containers as pods, in the same way that you would run any other containerized
service.  The topology would be simply:

Before adopting a 'canned' prometheus implementation ~ make sure you differentiate wether prometheus is being used
at the cluster level, or the application level.

At the cluster level:

- Running a prometheus master in a gloabl namespace (i.e. system or default).
- Running a prometheus nodes in a daemonSet of prometheus nodes that are privileged, having the
master scrape from the nodes.

At the application level: 

- Running a prometheus master in your apps namespace
- Having it scrape specifically from services which you care about in your app.

### The Prometheus Operator

The [Prometheus Operator](https://github.com/coreos/prometheus-operator)
provides managed Prometheus and Alertmanager on top of Kubernetes. See the
[README](https://github.com/coreos/prometheus-operator/blob/master/README.md)
for a list of functionalities it provides as well as instructions and guides on
how to use it.

As an introduction there is a [blog
post](https://coreos.com/blog/the-prometheus-operator.html) about it.

## The Prometheus Operator

The [Prometheus Operator](https://github.com/coreos/prometheus-operator)
provides managed Prometheus and Alertmanager on top of Kubernetes. See the
[README](https://github.com/coreos/prometheus-operator/blob/master/README.md)
for a list of functionalities it provides as well as instructions and guides on
how to use it.

As an introduction there is a [blog
post](https://coreos.com/blog/the-prometheus-operator.html) about it.

## The Prometheus Helm Chart

The [Prometheus Helm
Chart](https://github.com/kubernetes/charts/tree/master/stable/prometheus) is a
community maintained helm chart to customize and run Prometheus based on your
needs.
