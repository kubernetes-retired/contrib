# general AWS placement and authorization
variable "aws_region" {
  default = "us-west-2"
}

variable "aws_availability_zone" {
  default = "us-west-2a"
}

variable "aws_key_pair_pubkey" {
  description = "aws keypair pubkey contents"
}

variable "AWS_S3_REGION" {
  default = "us-east-1"
}

# Provisioning values

variable "KUBE_VERSION" {
  default = "v1.1.2"
}

variable "KUBE_PROXY_TOKEN" {}
variable "KUBE_USER" {
  default = "admin"
}
variable "KUBELET_TOKEN" {}
variable "KUBE_PASSWORD" {}

# Cluster identification and size

variable "CLUSTER_ID" {
  default = "kubernetes"
  description = "aka INSTANCE_PREFIX or KUBE_AWS_INSTANCE_PREFIX"
}

variable "MASTER_SIZE" {
  default = "t2.micro"
  description = "master node size"
}

variable "MINION_SIZE" {
  default = "t2.micro"
  description = "minion node size"
}

variable "NUM_MINIONS" {
  default = 3
  description = "number of minions"
}

variable "DOCKER_STORAGE" {
  default = "aufs"
}

variable "CLUSTER_IP_RANGE" {
  default = "10.244.0.0/16"
}

variable "MASTER_IP_RANGE" {
  default = "10.246.0.0/24"
}

variable "KUBE_RUNTIME_CONFIG" {
  default = ""
}

# Enable various v1beta1 features
variable "ENABLE_DEPLOYMENTS" {
  default = "false"
}
variable "ENABLE_DAEMONSETS" {
  default = "false"
}

# Optional: Cluster monitoring to setup as part of the cluster bring up:
#   none     - No cluster monitoring setup
#   influxdb - Heapster, InfluxDB, and Grafana
variable "ENABLE_CLUSTER_MONITORING" {
  default = "influxdb"
}

# Optional: Enable node logging.
variable "ENABLE_NODE_LOGGING" {
  default = "true"
}
variable "LOGGING_DESTINATION" {
  default = "elasticsearch"
}

# Optional: When set to true, Elasticsearch and Kibana will be setup as part of the cluster bring up.
variable "ENABLE_CLUSTER_LOGGING" {
  default = "true"
}

variable "ELASTICSEARCH_LOGGING_REPLICAS" {
  default = 1
}

variable "EXTRA_DOCKER_OPTS" {
  default = ""
}

variable "ENABLE_CLUSTER_DNS" {
  default = "true"
}
variable "DNS_SERVER_IP" {
  default = "10.0.0.10"
}

variable "DNS_DOMAIN" {
  default = "cluster.local"
}

variable "DNS_REPLICAS" {
  default = 1
}

# Optional: Install Kubernetes UI
variable "ENABLE_CLUSTER_UI" {
  default = "true"
}

# Optional: Create autoscaler for cluster's nodes.
variable "ENABLE_NODE_AUTOSCALER" {
  default = "false"
}

# Admission Controllers to invoke prior to persisting objects in cluster
variable "ADMISSION_CONTROL" {
  default = "NamespaceLifecycle,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota"
}

# Optional: Enable/disable public IP assignment for minions.
# Important Note: disable only if you have setup a NAT instance for internet access and configured appropriate routes!
variable "ENABLE_MINION_PUBLIC_IP" {
  default = "true"
}

# OS options for minions
variable "KUBE_OS_DISTRIBUTION" {
  default = "vivid"
  description = "choice of os distribution to use (coreos, trusty, vivid, wheezy, jessie)"
}
variable "KUBE_MINION_IMAGE" {
  default = ""
}
variable "COREOS_CHANNEL" {
  default = "alpha"
}

variable "CONTAINER_RUNTIME" {
  default = "docker"
}
