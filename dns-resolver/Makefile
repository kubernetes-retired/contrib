# Makefile for the Docker image gcr.io/google_containers/dns-resolver
# If you update this image please bump the tag value before pushing.

.PHONY: all container push

TAG = 0.7
PREFIX = aledbf
UNBOUND_IMAGE = contrib-unbound

all: container push

controller:
	CGO_ENABLED=0 GOOS=linux godep go build -a -ldflags '-w' -o ./rootfs/kube2dns

unbound:
	docker build -t $(UNBOUND_IMAGE):$(TAG) build
	docker create --name $(UNBOUND_IMAGE) $(UNBOUND_IMAGE):$(TAG) true
	# docker cp semantics changed between 1.7 and 1.8, so we cp the file to cwd and rename it.
	docker cp $(UNBOUND_IMAGE):/tmp/unbound-host_1.5.7-1ubuntu1_amd64.deb .
	docker cp $(UNBOUND_IMAGE):/tmp/unbound-anchor_1.5.7-1ubuntu1_amd64.deb .
	docker cp $(UNBOUND_IMAGE):/tmp/unbound_1.5.7-1ubuntu1_amd64.deb .
	docker cp $(UNBOUND_IMAGE):/tmp/python-unbound_1.5.7-1ubuntu1_amd64.deb .
	docker cp $(UNBOUND_IMAGE):/tmp/libunbound2_1.5.7-1ubuntu1_amd64.deb .
	docker rm -f $(UNBOUND_IMAGE)

container: unbound controller
	docker build -t $(PREFIX)/dns-resolver:$(TAG) .

push:
	gcloud docker push $(PREFIX)/dns-resolver:$(TAG)
