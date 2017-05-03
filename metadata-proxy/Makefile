.PHONY:	build push

# TAG is the version to build and push to.
PREFIX = gcr.io/google_containers
TAG = 1.0

build:
	# We explicitly add "--pull" flag to always fetch the latest version
	# of the base image. This is necessary to avoid using cached local
	# versions of image e.g. when updating insecure base images.
	docker build --pull -t ${PREFIX}/metadata-proxy:$(TAG) .

push: build
	gcloud docker -- push ${PREFIX}/metadata-proxy:$(TAG)
