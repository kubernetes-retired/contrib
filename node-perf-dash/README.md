# Kubernetes Node Performance Dashboard

Node Performance Dashboard is a web UI to collect and analyze performance test results of Kubernetes nodes. It collects data from Kubernetes node e2e performance tests, which can be stored either in local FS or Google GCS, then visualizes the data in 4 dashboards:

* **Builds**: monitoring performance change over different builds
* **Comparison**: compare performance change with different test parameters (e.g. pod number, creation speed, machine type)
* **Time series**: time series data including operation tracing probes inside kubernetes and resource-usage change over time
* **Tracing**: plot the latency percentile between any two tracing probes over different build
