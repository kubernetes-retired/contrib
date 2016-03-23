provider "aws" {
  access_key = "{{access_key_id}}"
  secret_key = "{{secret_access_key}}"
  region = "{{region}}"
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
{% for instance in master_instances %}
resource "aws_instance" "staging_master{{instance.number}}" {
  ami = "ami-ec46a283"
  instance_type = "t2.micro"
  user_data = "${file("master/assets/{{instance.number}}/cloud-config")}"
  private_ip = "{{instance.private_ip}}"
  vpc_security_group_ids = ["${aws_security_group.master.id}"]
  key_name = "{{aws_key_name}}"
  iam_instance_profile = "${aws_iam_instance_profile.admin_profile.id}"

  root_block_device {
    volume_size = 20
  }

  tags {
    Name = "Staging Master {{instance.number}}"
    Stack = "Staging"
    NodeType = "Master"
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
    source = "master/assets/{{instance.number}}/options.env"
    destination = "/etc/flannel/options.env"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/40-ExecStartPre-symlink.conf"
    destination = "/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/40-flannel.conf"
    destination = "/etc/systemd/system/docker.service.d/40-flannel.conf"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/kubelet.service"
    destination = "/etc/systemd/system/kubelet.service"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/kube-apiserver.service"
    destination = "/etc/systemd/system/kube-apiserver.service"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/kube-proxy.yaml"
    destination = "/etc/kubernetes/manifests/kube-proxy.yaml"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/kube-podmaster.yaml"
    destination = "/etc/kubernetes/manifests/kube-podmaster.yaml"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/kube-controller-manager.yaml"
    destination = "/srv/kubernetes/manifests/kube-controller-manager.yaml"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/kube-scheduler.yaml"
    destination = "/srv/kubernetes/manifests/kube-scheduler.yaml"
  }

  provisioner "file" {
    source = "common/assets/kubelet-wrapper"
    destination = "/opt/bin/kubelet-wrapper"
  }

  provisioner "file" {
    source = "master/assets/{{instance.number}}/etcd.client.conf"
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
    source = "master/assets/certificates/master{{instance.number}}-master-client.pem"
    destination = "/etc/kubernetes/ssl/master-client.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-client.pem"
    destination = "/etc/ssl/etcd/master-client.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-client-key.pem"
    destination = "/etc/kubernetes/ssl/master-client-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-client-key.pem"
    destination = "/etc/ssl/etcd/master-client-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-peer.pem"
    destination = "/etc/kubernetes/ssl/master-peer.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-peer.pem"
    destination = "/etc/ssl/etcd/master-peer.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-peer-key.pem"
    destination = "/etc/kubernetes/ssl/master-peer-key.pem"
  }

  provisioner "file" {
    source = "master/assets/certificates/master{{instance.number}}-master-peer-key.pem"
    destination = "/etc/ssl/etcd/master-peer-key.pem"
  }

  provisioner "file" {
    source = "master/kubectl-1.1.8"
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
{% for instance in worker_instances %}
resource "aws_instance" "staging_worker{{instance['number']}}" {
  ami = "ami-ec46a283"
  instance_type = "t2.micro"
  user_data = "${file("worker/assets/{{instance.number}}/cloud-config")}"
  private_ip = "{{instance.private_ip}}"
  vpc_security_group_ids  = ["${aws_security_group.worker.id}"]
  key_name = "{{aws_key_name}}"
  iam_instance_profile = "${aws_iam_instance_profile.admin_profile.id}"

  root_block_device {
    volume_size = 20
  }

  tags {
    Name = "Staging Worker {{instance.number}}"
    Stack = "Staging"
    NodeType = "Worker"
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
    source = "worker/assets/{{instance.number}}/options.env"
    destination = "/etc/flannel/options.env"
  }

  provisioner "file" {
    source = "worker/assets/{{instance.number}}/40-ExecStartPre-symlink.conf"
    destination = "/etc/systemd/system/flanneld.service.d/40-ExecStartPre-symlink.conf"
  }

  provisioner "file" {
    source = "worker/assets/{{instance.number}}/40-flannel.conf"
    destination = "/etc/systemd/system/docker.service.d/40-flannel.conf"
  }

  provisioner "file" {
    source = "worker/assets/{{instance.number}}/kubelet.service"
    destination = "/etc/systemd/system/kubelet.service"
  }

  provisioner "file" {
    source = "worker/assets/{{instance.number}}/kube-proxy.yaml"
    destination = "/etc/kubernetes/manifests/kube-proxy.yaml"
  }

  provisioner "file" {
    source = "common/assets/kubelet-wrapper"
    destination = "/opt/bin/kubelet-wrapper"
  }

  /*TODO: Make conditional*/
  provisioner "file" {
    source = "common/assets/fluentd-es.yaml"
    destination = "/etc/kubernetes/manifests/fluentd-es.yaml"
  }

  provisioner "file" {
    source = "worker/assets/certificates/ca.pem"
    destination = "/etc/ssl/etcd/ca.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker{{instance.number}}-worker-client.pem"
    destination = "/etc/ssl/etcd/worker.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker{{instance.number}}-worker-client-key.pem"
    destination = "/etc/ssl/etcd/worker-key.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/ca.pem"
    destination = "/etc/kubernetes/ssl/ca.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker{{instance.number}}-worker-client.pem"
    destination = "/etc/kubernetes/ssl/worker.pem"
  }

  provisioner "file" {
    source = "worker/assets/certificates/worker{{instance.number}}-worker-client-key.pem"
    destination = "/etc/kubernetes/ssl/worker-key.pem"
  }

  provisioner "remote-exec" {
    script = "worker/finalize.sh"
  }
}
{% endfor %}
