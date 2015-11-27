all: push

# 0.0.0 shouldn't clobber any released builds
TAG = 0.0
PREFIX = gcr.io/google_containers/haproxy
HAPROXY_IMAGE = haproxy

container: haproxy
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

haproxy:
	docker build -t $(HAPROXY_IMAGE):$(TAG) build
	docker create --name $(HAPROXY_IMAGE) $(HAPROXY_IMAGE):$(TAG) true
	# docker cp semantics changed between 1.7 and 1.8, so we cp the file to cwd and rename it.
	docker cp $(HAPROXY_IMAGE):/work/x86_64/haproxy-1.6-r2.apk .
	docker rm -f $(HAPROXY_IMAGE)
	mv haproxy-1.6-r2.apk haproxy.apk

clean:
	rm -f haproxy.apk
	# remove haproxy images
	docker rmi -f $(HAPROXY_IMAGE):$(TAG) || true
	docker rmi -f $(PREFIX):$(TAG) || true
