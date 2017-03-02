TAG=v1
IMAGE=gcr.io/google-containers/startup-script

.PHONY: build push

build:
	docker build --pull -t $(IMAGE):$(TAG) .

push: build
	gcloud docker -- push $(IMAGE):$(TAG)
