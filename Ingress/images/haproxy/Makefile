all: push

# 0.0.0 shouldn't clobber any released builds
TAG = 0.2
PREFIX = gcr.io/google_containers/haproxy
HAPROXY_IMAGE = haproxy

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	# remove haproxy images
	docker rmi -f $(PREFIX):$(TAG) || true
