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

# Cross-platform image that is based on busybox, but includes glibc so dynamically linked binaries may run inside. It's only 11 MB!
# make [ARCH=amd64] REGISTRY=[gcr.io/google_containers] (build|push)

.PHONY:	build push

REGISTRY?=gcr.io/google_containers
IMAGE=tiny-glibc
TAG=1.0
ARCH?=amd64
TEMP_DIR:=$(shell mktemp -d)
BIND_QEMU_VOLUME_ARG=-v $(TEMP_DIR)/qemu-$(QEMUARCH)-static:/usr/bin/qemu-$(QEMUARCH)-static

ifeq ($(ARCH),amd64)
	BUILDIMAGE?=debian:jessie
	BASEIMAGE?=busybox
	BIND_QEMU_VOLUME_ARG=
endif
ifeq ($(ARCH),arm)
	BUILDIMAGE?=armel/debian:jessie
	BASEIMAGE?=armel/busybox
	QEMUARCH=arm
endif
ifeq ($(ARCH),arm64)
	BUILDIMAGE?=aarch64/debian:jessie
	BASEIMAGE?=aarch64/busybox
	QEMUARCH=aarch64
endif
ifeq ($(ARCH),ppc64le)
	BUILDIMAGE?=ppc64le/debian:jessie
	BASEIMAGE?=ppc64le/busybox
	QEMUARCH=ppc64le
endif

build:
	cp ./* $(TEMP_DIR)
	cd $(TEMP_DIR) && sed -i "s|BASEIMAGE|$(BASEIMAGE)|g" Dockerfile

ifneq ($(ARCH),amd64)
	# Register /usr/bin/qemu-ARCH-static as the handler for ARM binaries in the kernel
	docker run --rm --privileged multiarch/qemu-user-static:register --reset
	curl -sSL https://github.com/multiarch/qemu-user-static/releases/download/v2.5.0/x86_64_qemu-$(QEMUARCH)-static.tar.xz | tar -xJ -C $(TEMP_DIR)
endif

	# Download glibc from the debian repositories and add them to a busybox image 
	docker run -it --rm -v $(TEMP_DIR):/build $(BIND_QEMU_VOLUME_ARG) $(BUILDIMAGE) /bin/bash -c \
		"apt-get update \
		&& apt-get download libc6 \
		&& mkdir libc \
		&& dpkg-deb -x *.deb libc \
		&& cd libc \
		&& tar -czf /build/glibc.tar.gz ."

	docker build -t $(REGISTRY)/$(IMAGE)-$(ARCH):$(TAG) $(TEMP_DIR)

	rm -rf $(TEMP_DIR)

push: build
	gcloud docker push $(REGISTRY)/$(IMAGE)-$(ARCH):$(TAG)

all: push
