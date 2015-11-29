Terraforming Kubernetes on AWS
==============================

Status: upstream v1.1.2 parity

This set of terraform descriptions aims to match the result of a fairly standard AWS deployment.

The goal of this project is effectively a terraform version of:

```sh
$ export KUBERNETES_PROVIDER=aws MASTER_RESERVED_IP=auto; curl -sS https://get.k8s.io | bash
```

I will accept contributions that add HA and other aspects but would like to remain close to upstream for educational purposes.

![terraform graph output](https://raw.github.com/tmc/k8s-terraform-aws-ubuntu/master/graph.png)

Prerequisites
-------------
You should have [terraform](https://www.terraform.io/downloads.html) installed. Your AWS_ACCESS_KEY and AWS_SECRET_ACCESS_KEY environment variables should be populated correctly.

You are expected to bring your own ssh keypair and it should be present in your current ssh-agent session.

`kubectl` should be on your $PATH.

Usage
-----
Clone this repository to experiment with it.

Basic invocation is controlled by the 'kube-up' target:
```sh
make kube-up
```

But this requires the input of the public key import as a keypair for the new instances so it will fail as such:
```sh
$ make kube-up
terraform apply -var-file=tokens.tfvars
There are warnings and/or errors related to your configuration. Please
fix these before continuing.

Errors:

  * 1 error(s) occurred:

  * Required variable not set: aws_key_pair_pubkey
  make: *** [kube-up] Error 1
```

Supply the contents of your public key to use in new cluster like so:
```sh
$ ssh-keygen -f "~/.ssh/kube_aws_rsa" -N ''
$ export TF_VAR_aws_key_pair_pubkey="$(cat ~/.ssh/kube_aws_rsa.pub)"
$ make kube-up
```

Parameters
----------------
See variables.tf for full list but a few interesting ones follow:

Supplying a custom Kubernetes version (currently defaults to v1.1.2)
```sh
$ TF_VAR_KUBE_VERSION=v1.1.2 make kube-up
```
This will create a wholly separate instance of a cluster with 'terranetes' as the prefix.



Supplying a custom cluster identifier:
```sh
$ TF_VAR_CLUSTER_ID=terranetes make kube-up
```
This will create a a cluster with 'terranetes' as the naming prefix and KubernetesCluster cluster name.


Credentials
-----------
On first invocation 'tokens.tfvars' is generated with some randomly generated values. Save this file.
