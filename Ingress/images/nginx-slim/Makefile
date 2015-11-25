all: push

# 0.0.0 shouldn't clobber any released builds
TAG = 0.0
PREFIX = gcr.io/google_containers/nginx-slim

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	docker rmi -f $(PREFIX):$(TAG) || true
