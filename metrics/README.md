Kubernetes devX metrics aggregator
==================================

Introduction
------------
The goal of this document is to describe the new binary that will pull all
the information needed to generate metrics for the Kubernetes project.

General concept
---------------
The aggregator is a Go program, running inside a docker container, within
the GKE utility cluster.

It is made of multiple consumer that are pulling the data from different
sources, and a consumer that pushes that data to the external time-series
database. This is done through a Go channel.

Each consumer can decide the pace of pulling.

Push object
-----------
The push object has a dictionary of key: value pairs, indicating metrics and
their values at a given time. It can optionally contain an instance ID, if
it’s monitoring multiple resources.  The Stackdriver API accepts data point
with {name, value, collected_at, instance [optional]}.

Producer plugins
----------------
Each type of producer plugin is pulling data from a different source. It can
then create a generic push object from that information, and push that to
the consumer through a channel.

Consumer plugins
----------------
A consumer is simply an object reading push objects through a channel, and
pushing them to the time-series database. We can have multiple types of
consumer, each configured to send to a different database.

Time series database example (would be implemented as pusher plugins):
- Stackdriver (built-in support for GCP)
- Prometheus? (part of CNCF)
- InfluxDB? (better ‘event’ support?)
- BigTable? ( https://cloud.google.com/bigtable/docs/schema-design-time-series )

Reference
---------
Example code: https://support.stackdriver.com/customer/portal/articles/1491766-sending-custom-application-metrics-to-the-stackdriver-system
