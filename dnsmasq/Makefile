# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Makefile for the Docker image gcr.io/google_containers/kube2sky
# MAINTAINER: Abhishek Shah <abshah@google.com>
# If you update this image please bump the tag value before pushing.
#
# Usage:
#   [TAG=1.0] [REGISTRY=gcr.io/google_containers] [NAMESERVER=127.0.0.1#10053] make container

# Default registry, arch and tag. This can be overwritten by arguments to make
TAG?=1.0
REGISTRY?=gcr.io/google_containers
NAMESERVER=127.0.0.1\#10053

KUBE_ROOT=$(shell pwd)/..
TEMP_DIR:=$(shell mktemp -d)

all: container

container:
	cp $(KUBE_ROOT)/dnsmasq/Dockerfile $(TEMP_DIR)
	cp $(KUBE_ROOT)/dnsmasq/dnsmasq.conf $(TEMP_DIR)
	cp $(KUBE_ROOT)/dnsmasq/default $(TEMP_DIR)

	# Replace Nameserver in the dnsmasq.conf file
	cd $(TEMP_DIR) && sed -i "s|__NAMESERVER__|$(NAMESERVER)|g" dnsmasq.conf

	# And build the image
	docker build -t $(REGISTRY)/dnsmasq:$(TAG) $(TEMP_DIR)

	# delete temp dir
	#rm -rf $(TEMP_DIR)
	echo $(TEMP_DIR)

push: container
	ifeq ($(REGISTRY),gcr.io/google_containers)
		gcloud docker push $(REGISTRY)/dnsmasq:$(TAG)
	else
		docker push $(REGISTRY)/dnsmasq:$(TAG)
	endif

.PHONY: all container push
