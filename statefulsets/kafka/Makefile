VERSION=v1
PROJECT_ID=google_samples
PROJECT=gcr.io/${PROJECT_ID}

all: build

build:
	docker build --pull -t ${PROJECT}/k8skafka:${VERSION} .

push: build
	gcloud docker -- push ${PROJECT}/k8skafka:${VERSION}

.PHONY: all build push
