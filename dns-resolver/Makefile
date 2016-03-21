# Makefile for the Docker image gcr.io/google_containers/dns-resolver
# If you update this image please bump the tag value before pushing.

.PHONY: all container push

TAG = 0.3
PREFIX = aledbf

all: container push

controller:
	CGO_ENABLED=0 GOOS=linux godep go build -a -ldflags '-w' -o ./rootfs/kube2dns

container: controller
	docker build -t $(PREFIX)/dns-resolver:$(TAG) .

push:
	gcloud docker push $(PREFIX)/dns-resolver:$(TAG)
