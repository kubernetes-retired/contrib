variable "access_key" {}
variable "secret_key" {}
variable "region" {}
variable "master_ip" {}
variable "worker_ip" {}
variable "aws_key_name" {}
variable "dns_address" {}
variable "public_ip" {}

provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
}

resource "aws_iam_instance_profile" "admin_profile" {
  name = "admin_profile"
  roles = ["${aws_iam_role.admin.name}"]
}

resource "aws_iam_role_policy" "admin_policy" {
  name = "admin_policy"
  role = "${aws_iam_role.admin.id}"
  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "*",
      "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_role" "admin" {
  name = "admin_role"
  path = "/"
  assume_role_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Action": "sts:AssumeRole",
        "Principal": {"AWS": "*"},
        "Effect": "Allow",
        "Sid": ""
      }
    ]
}
EOF
}

resource "aws_instance" "staging_master" {
  ami = "ami-ec46a283"
  instance_type = "t2.small"
  user_data = "${file("master/assets/cloud-config")}"
  private_ip = "${var.master_ip}"
  vpc_security_group_ids = ["${aws_security_group.master.id}"]
  key_name = "${var.aws_key_name}"
  iam_instance_profile = "${aws_iam_instance_profile.admin_profile.id}"

  root_block_device {
    volume_size = 20
  }

  tags {
    Name = "Staging Master"
    Stack = "Staging"
    NodeType = "Master"
  }

  provisioner "local-exec" {
    command = "./master/generate-assets.py ${var.dns_address} ${var.region} ${var.public_ip} ${aws_instance.staging_master.private_ip}"
  }

  connection {
    user = "core"
    type = "ssh"
    private_key = "${file("~/.ssh/id_rsa")}"
  }

  provisioner "remote-exec" {
    script = "master/bootstrap.sh"
  }

  provisioner "file" {
    source = "master/assets/options.env"
    destination = "/etc/flannel/options.env"
  }

  provisioner "file" {
    source = "master/assets/40-ExecStartPre-symlink.conf"
    destination = "/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf"
  }

  provisioner "file" {
    source = "master/assets/40-flannel.conf"
    destination = "/etc/systemd/system/docker.service.d/40-flannel.conf"
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
    destination = "/opt/local/bin/kubelet-wrapper"
  }

  provisioner "file" {
    source = "master/assets/etcd.client.conf"
    destination = "/etc/kubernetes/etcd.client.conf"
  }

  provisioner "file" {
    source = "master/assets/kube.conf"
    destination = "/etc/kubernetes/kube.conf"
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
    source = "master/assets/certificates/master1-master-client.pem"
    destination = "/etc/kubernetes/ssl/master-client.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-client.pem"
    destination = "/etc/ssl/etcd/master-client.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-client-key.pem"
    destination = "/etc/kubernetes/ssl/master-client-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-client-key.pem"
    destination = "/etc/ssl/etcd/master-client-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-peer.pem"
    destination = "/etc/kubernetes/ssl/master-peer.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-peer.pem"
    destination = "/etc/ssl/etcd/master-peer.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-peer-key.pem"
    destination = "/etc/kubernetes/ssl/master-peer-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master1-master-peer-key.pem"
    destination = "/etc/ssl/etcd/master-peer-key.pem"
  }

  provisioner "file" {
    source = "master/kubectl-1.1.8"
    destination = ".local/bin/kubectl"
  }

  provisioner "file" {
    source = "master/addons/dns-addon.yaml"
    destination = "/etc/kubernetes/addons/dns-addon.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "common/assets/fluentd-es.yaml"
    destination = "/etc/kubernetes/manifests/fluentd-es.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "master/addons/es-controller.yaml"
    destination = "/etc/kubernetes/addons/es-controller.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "master/addons/es-service.yaml"
    destination = "/etc/kubernetes/addons/es-service.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "master/addons/kibana-controller.yaml"
    destination = "/etc/kubernetes/addons/kibana-controller.yaml"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "master/addons/kibana-service.yaml"
    destination = "/etc/kubernetes/addons/kibana-service.yaml"
  }

  /*TODO: Parameterize somehow!*/
  provisioner "file" {
    source = "quay-io-secret.yaml"
    destination = "/tmp/quay-io-secret.yaml"
  }

  provisioner "remote-exec" {
    script = "master/finalize.sh"
  }
}

resource "aws_security_group" "master" {
  name = "master"
  description = "Allow traffic to Kubernetes master"

  ingress {
    from_port = 22
    to_port = 22
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 443
    to_port = 443
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 10250
    to_port = 10250
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 2379
    to_port = 2379
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 2380
    to_port = 2380
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
      from_port = 0
      to_port = 0
      protocol = "-1"
      cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "worker" {
  name = "worker"
  description = "Allow traffic to Kubernetes worker"

  ingress {
    from_port = 22
    to_port = 22
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 443
    to_port = 443
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 10250
    to_port = 10250
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
      from_port = 0
      to_port = 0
      protocol = "-1"
      cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "staging_worker" {
  ami = "ami-ec46a283"
  instance_type = "t2.small"
  user_data = "${file("worker/assets/cloud-config")}"
  private_ip = "${var.worker_ip}"
  vpc_security_group_ids  = ["${aws_security_group.worker.id}"]
  key_name = "${var.aws_key_name}"
  iam_instance_profile = "${aws_iam_instance_profile.admin_profile.id}"

  root_block_device {
    volume_size = 20
  }

  tags {
    Name = "Staging Worker"
    Stack = "Staging"
    NodeType = "Worker"
  }

  provisioner "local-exec" {
    command = "./worker/generate-assets.py ${aws_instance.staging_worker.private_ip} ${aws_instance.staging_master.private_ip}"
  }

  connection {
    user = "core"
    type = "ssh"
    private_key = "${file("~/.ssh/id_rsa")}"
  }

  provisioner "remote-exec" {
    script = "worker/bootstrap.sh"
  }

  provisioner "file" {
    source = "worker/assets/options.env"
    destination = "/etc/flannel/options.env"
  }

  provisioner "file" {
    source = "worker/assets/40-ExecStartPre-symlink.conf"
    destination = "/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf"
  }

  provisioner "file" {
    source = "worker/assets/40-flannel.conf"
    destination = "/etc/systemd/system/docker.service.d/40-flannel.conf"
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
    destination = "/opt/local/bin/kubelet-wrapper"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "common/assets/fluentd-es.yaml"
    destination = "/etc/kubernetes/manifests/fluentd-es.yaml"
  }

  provisioner "file" {
    source = "worker/assets/kube.conf"
    destination = "/etc/kubernetes/kube.conf"
  }

  provisioner "file" {
    source = "worker/assets/certificates/ca.pem"
    destination = "/etc/ssl/etcd/ca.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker1-worker-client.pem"
    destination = "/etc/ssl/etcd/worker.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker1-worker-client-key.pem"
    destination = "/etc/ssl/etcd/worker-key.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/ca.pem"
    destination = "/etc/kubernetes/ssl/ca.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker1-worker-client.pem"
    destination = "/etc/kubernetes/ssl/worker.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker1-worker-client-key.pem"
    destination = "/etc/kubernetes/ssl/worker-key.pem"
  }

  provisioner "remote-exec" {
    script = "worker/finalize.sh"
  }
}
