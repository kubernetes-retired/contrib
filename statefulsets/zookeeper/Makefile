VERSION=v2
PROJECT_ID=google_samples
PROJECT=gcr.io/${PROJECT_ID}

all: build

build:
	docker build -t ${PROJECT}/k8szk:${VERSION} .

push: build
	gcloud docker -- push ${PROJECT}/k8szk:${VERSION}

.PHONY: all build push
