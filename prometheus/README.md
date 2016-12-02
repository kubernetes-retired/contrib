# Prometheus in Kubernetes

There are two maintained ways to run Prometheus in Kubernetes:

* [The Prometheus Operator](#the-prometheus-operator)
* [The Prometheus Helm Chart](#the-prometheus-helm-chart)

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
