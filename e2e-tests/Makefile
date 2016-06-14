all: container

DATE := $(shell date +%F)
GIT := $(shell git rev-parse --short HEAD)

TAG ?= $(DATE)-$(GIT)

DEFAULTREPO := gcr.io/google_containers
REPO ?= $(DEFAULTREPO)
APP ?= test-manager
CONTAINER := $(REPO)/$(APP):$(TAG)

KUBECONFIG ?= $(HOME)/.kube/config

# just build the binary
build:
	GOBIN=$(PWD) CGO_ENABLED=0 GOOS=linux go install -installsuffix cgo -ldflags '-w'

test:
	CGO_ENABLED=0 GOOS=linux go test ./...

# build the container with the binary
container: build
	docker build -t $(CONTAINER) .

# push the container
push: container
ifneq (,$(findstring gcr.io,$(REPO)))
	gcloud docker push $(CONTAINER)
else
	docker push $(CONTAINER)
endif

# Launch the container on a cluster
# The cluster will likely need a service to get access to the web interface (see service.yaml)
deploy: push deployment
	# Deploy the new deployment
	kubectl --kubeconfig=$(KUBECONFIG) apply -f local.deployment.yaml --record

# Try to run the binary locally using docker, doesn't need to push or have a running kube cluster.
# Binary is exposed on port 8080
local_dryrun: container
	docker run -t --rm -p 8082:8082 -p 9999:9999 $(CONTAINER)

# updates the deployment.yaml with current build information and sets it to --dry-run
deployment:
	# update the deployment.yaml with the current date and git hash
	sed -e 's|[[:digit:]]\{4\}-[[:digit:]]\{2\}-[[:digit:]]\{2\}-[[:xdigit:]]\+|$(TAG)|g' deployment.yaml > local.deployment.yaml
	# update the deployment.yaml with the current repo
	sed -i -e 's|gcr.io/google_containers|$(REPO)|g' local.deployment.yaml

clean:
	rm -f e2e-tests local.deployment.yaml

help:
	@echo "ENVIRONMENT VARS:"
	@echo " REPO=       repository for the docker image being build. Default: $(REPO)"
	@echo " KUBECONFIG= kubeconfig file for deployment. Default: $(KUBECONFIG)"
	@echo " READONLY=   should the container actually mute github objects or just do everything else. Default: $(READONLY)"
	@echo " APP=        which application you are trying to deploy. cherrypick or submit-queue. Default: $(APP)"
	@echo ""
	@echo "TARGETS:"
	@echo " all:          runs 'container'"
	@echo " build:        builds the binary"
	@echo " container:    builds the binary and creates a container with the binary"
	@echo " push:         pushes the container to the registry"
	@echo " deploy:       launches/updates the app on a kubernetes cluster"
	@echo " local_dryrun: tries to launch the container locally with docker"
	@echo " deployment:   updates deployment.yaml and places results in local.deployment.yaml"
	@echo " clean:        deletes the binary and local files (does not delete old containers)"


.PHONY: all build test container push dryrun cleandryrun local_dryrun deployment clean help
