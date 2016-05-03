TAG=v1
IMAGE=gcr.io/google_containers/startup-script

.PHONY: build push

build:
	docker build -t $(IMAGE):$(TAG) .

push: build
	gcloud docker push $(IMAGE):$(TAG)
