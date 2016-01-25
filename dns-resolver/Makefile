# Makefile for the Docker image gcr.io/google_containers/dns-resolver
# If you update this image please bump the tag value before pushing.

.PHONY: all container push

TAG = 0.4
PREFIX = aledbf
UNBOUND_IMAGE = contrib-unbound

all: container push

controller:
	CGO_ENABLED=0 GOOS=linux godep go build -a -ldflags '-w' -o ./rootfs/kube2dns

unbound:
	docker build -t $(UNBOUND_IMAGE):$(TAG) build
	docker create --name $(UNBOUND_IMAGE) $(UNBOUND_IMAGE):$(TAG) true
	# docker cp semantics changed between 1.7 and 1.8, so we cp the file to cwd and rename it.
	docker cp $(UNBOUND_IMAGE):/work/x86_64/unbound-1.5.7-r1.apk .
	docker rm -f $(UNBOUND_IMAGE)
	mv unbound-1.5.7-r1.apk unbound.apk

container: unbound controller
	docker build -t $(PREFIX)/dns-resolver:$(TAG) .

push:
	gcloud docker push $(PREFIX)/dns-resolver:$(TAG)
