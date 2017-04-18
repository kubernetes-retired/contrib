ImageBuilder
============

ImageBuilder is a tool for building an optimized k8s images, currently only supporting AWS.

It is a wrapper around bootstrap-vz (the tool used to build official Debian cloud images).  
It adds functionality to spin up an instance for building the image, and publishing the image to all regions.

To use:

```
go install k8s.io/contrib/imagebuilder

vpc_id=vpc-<your__vpc>
subnet_id=subnet-<your_subnet_id>
aws ec2 create-security-group --vpc-id ${vpc_id} --group-name imagebuilder --description "Group for imagebuilder"
security_group_id=???
aws ec2 authorize-security-group-ingress --group-id ${security_group_id}  --cidr 0.0.0.0/0 --protocol tcp --port 22
aws ec2 import-key-pair --key-name imagebuilder --public-key-material file:///${HOME}/.ssh/id_rsa.pub
```


Make sure your key is in ssh-agent:

```
eval `ssh-agent`
ssh-add ~/.ssh/id_rsa
```

Run the image builder:
```
~/k8s/bin/imagebuilder \
  --template ~/k8s/src/k8s.io/kubernetes/cluster/cloudimages/k8s-ebs-jessie-amd64-hvm.yml \
  --securitygroup ${security_group_id} \
  --subnet ${subnet_id} \
  --sshkey imagebuilder --v=2
```

This will create an instance to build the image, build the image as specified by `--template`, make the
image public and copy it to all accessible regions, and the shut down the builder instance.  Each of these stages
can be controlled through flags (for example, you might not want use `--publish=false` for an internal image.)

It will print the IDs of the image in each region, but it will also tag the image with a Name
as specified in the template) and this is the easier way to retrieve the image.

Advanced options
================

Check out `--help`, but these options control which operations we perform,
and may be useful for debugging or publishing a lot of images:

* `--up=true/false`, `--down=true/false` control whether we try to create and terminate an instance to do the building

* `--publish=true/false` controls whether we make the image public

* `--replicate=true/false` controls whether we copy the image to all regions

* `--image=ami-XXXXXX` use an alternative image for building.  This has been tested with the (Debian 8.1 Images)[https://wiki.debian.org/Cloud/AmazonEC2Image/Jessie]