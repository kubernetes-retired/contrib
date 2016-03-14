all: push

# 0.0 shouldn't clobber any released builds
TAG ?= 0.4
PREFIX ?= gcr.io/google_containers/servicelb
GCLOUD ?= gcloud
HAPROXY_IMAGE = contrib-haproxy
SRC = service_loadbalancer.go loadbalancer_log.go

service_loadbalancer: $(SRC)
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o $@ $(SRC)

container: service_loadbalancer

	docker build -t $(PREFIX):$(TAG) .

push: container
	$(GCLOUD) docker push $(PREFIX):$(TAG)

haproxy:
	docker build -t $(HAPROXY_IMAGE):$(TAG) build
	docker create --name $(HAPROXY_IMAGE) $(HAPROXY_IMAGE):$(TAG) true
	# docker cp semantics changed between 1.7 and 1.8, so we cp the file to cwd and rename it.
	docker cp $(HAPROXY_IMAGE):/work/x86_64/haproxy-1.6-r3.apk .
	docker rm -f $(HAPROXY_IMAGE)
	mv haproxy-1.6-r3.apk haproxy.apk

clean:
	rm -f service_loadbalancer haproxy.apk
	# remove servicelb and contrib-haproxy images
	docker rmi -f $(HAPROXY_IMAGE):$(TAG) || true
	docker rmi -f $(PREFIX):$(TAG) || true
	rm -f service_loadbalancer

.PHONY: container push
