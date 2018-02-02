all: push

# TAG 0.0 shouldn't clobber any release builds
TAG = 1.3
PREFIX = staging-k8s.gcr.io/echoserver-redirect

container:
	docker build --pull -t $(PREFIX):$(TAG) .

push: container
	gcloud docker -- push $(PREFIX):$(TAG)
