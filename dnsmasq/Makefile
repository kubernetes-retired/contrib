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

# Makefile for the Docker image gcr.io/google_containers/dnsmasq
# MAINTAINER: Abhishek Shah <abshah@google.com>
# If you update this image please bump the tag value before pushing.
#
# Usage:
#   [TAG=1.0] [REGISTRY=gcr.io/google_containers] [NAMESERVER=127.0.0.1#10053] make container

# Default registry, arch and tag. This can be overwritten by arguments to make
TAG?=1.2
REGISTRY?=gcr.io/google_containers
ARCH?=amd64
KUBE_CROSS_IMAGE:=$(REGISTRY)/kube-cross:v1.6.2-2
DNSMASQ_VERSION?=dnsmasq-2.76
TEMP_DIR:=$(shell mktemp -d)

ifeq ($(ARCH),amd64)
    BASEIMAGE?=alpine:3.4
else
    BASEIMAGE?=$(REGISTRY)/tiny-glibc-$(ARCH):1.0
endif

ifeq ($(ARCH),arm)
    TRIPLE?=arm-linux-gnueabi
endif
ifeq ($(ARCH),arm64)
    TRIPLE?=aarch64-linux-gnu
endif
ifeq ($(ARCH),ppc64le)
    TRIPLE?=powerpc64le-linux-gnu
endif

all: build

build:
ifeq ($(ARCH),amd64)
	cp Dockerfile $(TEMP_DIR)
else
	cp Dockerfile.cross $(TEMP_DIR)/Dockerfile
	sed -i "s|BASEIMAGE|$(BASEIMAGE)|g" $(TEMP_DIR)/Dockerfile

	# Build the dnsmasq binary using the cross-compilation docker image
	docker run -it --rm -v $(TEMP_DIR):/build $(KUBE_CROSS_IMAGE) /bin/bash -c \
	"curl -sSL  http://www.thekelleys.org.uk/dnsmasq/$(DNSMASQ_VERSION).tar.xz | tar -xJ \
	&& cd $(DNSMASQ_VERSION) \
	&& CC=$(TRIPLE)-gcc make \
	&& cp src/dnsmasq /build"
endif
	cp dnsmasq.conf $(TEMP_DIR)
	docker build -t $(REGISTRY)/kube-dnsmasq-$(ARCH):$(TAG) $(TEMP_DIR)
	rm -rf $(TEMP_DIR)

push: build
ifeq ($(REGISTRY),gcr.io/google_containers)
	gcloud docker push $(REGISTRY)/kube-dnsmasq-$(ARCH):$(TAG)
else
	docker push $(REGISTRY)/kube-dnsmasq-$(ARCH):$(TAG)
endif

.PHONY: push
