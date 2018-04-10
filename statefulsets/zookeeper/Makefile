VERSION=v3
PROJECT_ID=google_samples
PROJECT=gcr.io/${PROJECT_ID}

IMAGE_NAME = k8szk

ARCH ?= $(shell go env GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le s390x

MULTI_ARCH_IMG = $(PROJECT)/$(IMAGE_NAME)-$(ARCH)

QEMUVERSION=v2.9.1

BASEIMAGE ?= ubuntu:16.04

QEMUARCH ?= amd64

J_MULTI ?= /usr/lib/jvm/java-8-openjdk-amd64

# Set the baseimage for each arch
ifeq ($(ARCH), arm)
	BASEIMAGE = arm32v7/ubuntu:16.04
	QEMUARCH = arm
	J_MULTI = /usr/lib/jvm/java-8-openjdk-armhf
endif
ifeq ($(ARCH), arm64)
	BASEIMAGE = arm64v8/ubuntu:16.04
	QEMUARCH = aarch64
	J_MULTI = /usr/lib/jvm/java-8-openjdk-arm64
endif
ifeq ($(ARCH), ppc64le)
	BASEIMAGE = ppc64le/ubuntu:16.04
	QEMUARCH = ppc64le
	J_MULTI = /usr/lib/jvm/java-8-openjdk-ppc64el
endif
ifeq ($(ARCH), s390x)
	BASEIMAGE = s390x/ubuntu:16.04
	QEMUARCH = s390x
	J_MULTI = /usr/lib/jvm/java-8-openjdk-s390x
endif
	
TEMP_DIR := $(shell mktemp -d)

all: all-container

sub-container-%:
	$(MAKE) --no-print-directory ARCH=$* container

sub-push-%:
	$(MAKE) --no-print-directory ARCH=$* push

all-container: $(addprefix sub-container-,$(ALL_ARCH))

all-push: $(addprefix sub-push-,$(ALL_ARCH))

container: .container-$(ARCH)
.container-$(ARCH): 
	cp -r * $(TEMP_DIR)
	cd $(TEMP_DIR) && sed -i 's|BASEIMAGE|$(BASEIMAGE)|g' Dockerfile
	cd $(TEMP_DIR) && sed -i "s|ARCH|$(QEMUARCH)|g" Dockerfile
	cd $(TEMP_DIR) && sed -i "s|J_MULTI|$(J_MULTI)|g" Dockerfile

ifeq ($(ARCH),amd64)
	# When building "normally" for amd64, remove the whole line, it has no part in the amd64 image
	cd $(TEMP_DIR) && sed -i "/CROSS_BUILD_/d" Dockerfile
else
	# When cross-building, only the placeholder "CROSS_BUILD_" should be removed
	# Register /usr/bin/qemu-ARCH-static as the handler for ARM binaries in the kernel
	docker run --rm --privileged multiarch/qemu-user-static:register --reset
	curl -sSL https://github.com/multiarch/qemu-user-static/releases/download/$(QEMUVERSION)/x86_64_qemu-$(QEMUARCH)-static.tar.gz | tar -xz -C $(TEMP_DIR)
	cd $(TEMP_DIR) && sed -i "s/CROSS_BUILD_//g" Dockerfile
endif
	docker build --pull -t ${MULTI_ARCH_IMG}:$(VERSION) $(TEMP_DIR)
ifeq ($(ARCH), amd64)
	# Adding check for amd64
	docker tag $(MULTI_ARCH_IMG):$(VERSION) $(PROJECT)/$(IMAGE_NAME):$(VERSION)
endif


push: .push-$(ARCH)
.push-$(ARCH): .container-$(ARCH)
	gcloud docker -- push $(MULTI_ARCH_IMG):$(VERSION)
ifeq ($(ARCH), amd64)
	gcloud docker -- push $(IMAGE):$(VERSION)
endif


.PHONY: all build push
