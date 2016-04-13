provider "google" {
  credentials = "${file("account.json")}"
  project = "{{project}}"
  region = "{{region}}"
}

resource "google_compute_firewall" "default_internal" {
  name = "default-default-internal"
  network = "default"
  source_ranges = ["10.0.0.0/8"]

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports = ["1-65535"]
  }

  allow {
    protocol = "udp"
    ports = ["1-65535"]
  }
}

resource "google_compute_firewall" "default_ssh" {
  name = "default-default-ssh"
  network = "default"
  source_ranges = ["0.0.0.0/0"]

  allow {
    protocol = "tcp"
    ports = ["22"]
  }
}

resource "google_compute_firewall" "master_https" {
  name = "kubernetes-master-https"
  network = "default"
  target_tags = ["master"]
  source_ranges = ["0.0.0.0/0"]

  allow {
    protocol = "tcp"
    ports = ["443"]
  }
}

{% for instance in master_instances %}
resource "google_compute_instance" "staging_master{{instance.number}}" {
  name = "staging-master{{instance.number}}"
  machine_type = "g1-small"
  zone = "{{zone}}"
  tags = ["staging", "master"]
  description = "Staging master {{instance.number}}"
  can_ip_forward = true

  connection {
    user = "core"
    type = "ssh"
    private_key = "${file("~/.ssh/id_rsa")}"
  }

  metadata {
    user-data = "${file("master/assets/cloud-config")}"
    cluster-name = "Kubernetes"
  }

  network_interface {
    network = "default"

    access_config {
      nat_ip = "{{instance.public_ip}}"
    }
  }

  disk {
    image = "coreos-beta-991-1-0-v20160324"
    type = "pd-ssd"
    size = 20
  }

  service_account {
    scopes = ["storage-ro", "compute-rw", "monitoring", "logging-write"]
  }

  scheduling {
    on_host_maintenance = "MIGRATE"
    automatic_restart = true
  }

  provisioner "remote-exec" {
    script = "master/bootstrap.sh"
  }

  provisioner "file" {
    source = "master/assets/kubelet.service"
    destination = "/etc/systemd/system/kubelet.service"
  }

  provisioner "file" {
    source = "master/assets/kube-apiserver.service"
    destination = "/etc/systemd/system/kube-apiserver.service"
  }

  provisioner "file" {
    source = "master/assets/kube-proxy.yaml"
    destination = "/etc/kubernetes/manifests/kube-proxy.yaml"
  }

  provisioner "file" {
    source = "master/assets/kube-podmaster.yaml"
    destination = "/etc/kubernetes/manifests/kube-podmaster.yaml"
  }

  provisioner "file" {
    source = "master/assets/kube-controller-manager.yaml"
    destination = "/srv/kubernetes/manifests/kube-controller-manager.yaml"
  }

  provisioner "file" {
    source = "master/assets/kube-scheduler.yaml"
    destination = "/srv/kubernetes/manifests/kube-scheduler.yaml"
  }

  provisioner "file" {
    source = "common/assets/kubelet-wrapper"
    destination = "/opt/bin/kubelet-wrapper"
  }

  provisioner "file" {
    source = "master/assets/etcd.client.conf"
    destination = "/etc/kubernetes/etcd.client.conf"
  }

  provisioner "file" {
    source = "master/assets/certificates/ca.pem"
    destination = "/etc/kubernetes/ssl/ca.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/ca.pem"
    destination = "/etc/ssl/etcd/ca.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/mastermaster-client.pem"
    destination = "/etc/kubernetes/ssl/master-client.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-client.pem"
    destination = "/etc/ssl/etcd/master-client.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-client-key.pem"
    destination = "/etc/kubernetes/ssl/master-client-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-client-key.pem"
    destination = "/etc/ssl/etcd/master-client-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-peer.pem"
    destination = "/etc/kubernetes/ssl/master-peer.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-peer.pem"
    destination = "/etc/ssl/etcd/master-peer.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-peer-key.pem"
    destination = "/etc/kubernetes/ssl/master-peer-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master-peer-key.pem"
    destination = "/etc/ssl/etcd/master-peer-key.pem"
  }

  provisioner "file" {
    source = "master/kubectl-1.2.0"
    destination = "/opt/bin/kubectl"
  }

  provisioner "remote-exec" {
    script = "master/finalize.sh"
  }
  {% if instance.number == 1 %}
  /*TODO: Make conditional*/
  provisioner "file" {
    source = "master/addons/dns-addon.yaml"
    destination = "/tmp/dns-addon.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "common/assets/fluentd-es.yaml"
    destination = "/etc/kubernetes/manifests/fluentd-es.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "master/addons/fluentd-elasticsearch/"
    destination = "/tmp/"
  }

  provisioner "file" {
    source = "master/addons/cluster-monitoring/"
    destination = "/tmp/"
  }

  /*TODO: Parameterize somehow!*/
  provisioner "file" {
    source = "master/assets/quay-io-secret.yaml"
    destination = "/tmp/quay-io-secret.yaml"
  }

  provisioner "remote-exec" {
    script = "master/oneshot-finalize.sh"
  }
  {% endif %}
}
{% endfor %}
resource "google_compute_firewall" "node_all" {
  name = "kubernetes-node-all"
  network = "default"
  source_ranges = ["10.244.0.0/16"]
  target_tags = ["node"]

  allow {
    protocol = "tcp"
  }

  allow {
    protocol = "udp"
  }

  allow {
    protocol = "esp"
  }

  allow {
    protocol = "ah"
  }

  allow {
    protocol = "sctp"
  }
}

resource "google_compute_instance_template" "node" {
  name = "kubernetes-node-template"
  machine_type = "g1-small"
  can_ip_forward = true
  tags = ["staging", "node"]

  disk {
    source_image = "coreos-beta-991-1-0-v20160324"
    auto_delete = true
    boot = true
    disk_type = "pd-ssd"
    disk_size_gb = 20
  }

  network_interface {
    network = "default"
  }

  metadata {
    user-data = "${file("worker/assets/cloud-config")}"
  }

  service_account {
    scopes = ["compute-rw", "monitoring", "logging-write", "storage-ro"]
  }

  scheduling {
    on_host_maintenance = "MIGRATE"
    automatic_restart = true
  }

  provisioner "remote-exec" {
    script = "worker/bootstrap.sh"
  }

  provisioner "file" {
    source = "worker/assets/kubelet.service"
    destination = "/etc/systemd/system/kubelet.service"
  }

  provisioner "file" {
    source = "worker/assets/kube-proxy.yaml"
    destination = "/etc/kubernetes/manifests/kube-proxy.yaml"
  }

  provisioner "file" {
    source = "common/assets/kubelet-wrapper"
    destination = "/opt/bin/kubelet-wrapper"
  }

  provisioner "file" {
    source = "worker/assets/certificates/ca.pem"
    destination = "/etc/ssl/etcd/ca.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker-client.pem"
    destination = "/etc/ssl/etcd/worker.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker-client-key.pem"
    destination = "/etc/ssl/etcd/worker-key.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/ca.pem"
    destination = "/etc/kubernetes/ssl/ca.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker-client.pem"
    destination = "/etc/kubernetes/ssl/worker.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker-client-key.pem"
    destination = "/etc/kubernetes/ssl/worker-key.pem"
  }

  provisioner "remote-exec" {
    script = "worker/finalize.sh"
  }

  connection {
    user = "core"
    type = "ssh"
    private_key = "${file("~/.ssh/id_rsa")}"
  }
}

resource "google_compute_instance_group_manager" "node_group_1" {
  name = "staging-node-group-1"
  base_instance_name = "staging-node"
  target_size = {{num_nodes}}
  instance_template = "${google_compute_instance_template.node.self_link}"
  zone = "{{zone}}"

  connection {
    user = "core"
    type = "ssh"
    private_key = "${file("~/.ssh/id_rsa")}"
  }
}
