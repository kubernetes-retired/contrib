
FLAGS =
ENVVAR = GOOS=linux GOARCH=amd64 CGO_ENABLED=0
REGISTRY = gcr.io/google-containers
TAG = v0.3.1
ARCH ?= $(shell go env GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le


IMAGE = $(REGISTRY)/rescheduler
MULTI_ARCH_IMG = $(IMAGE)-$(ARCH)

BASEIMAGE ?= busybox:latest

ifeq ($(ARCH),arm)
        BASEIMAGE=armhf/busybox:latest
endif
ifeq ($(ARCH),arm64)
        BASEIMAGE=arm64v8/busybox:latest
endif
ifeq ($(ARCH),ppc64le)
        BASEIMAGE=ppc64le/busybox:latest
endif

deps:
	go get github.com/tools/godep

build: clean deps
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 godep go build ./...
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 godep go build -o rescheduler

test-unit: clean deps build
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 godep go test --test.short -race ./... $(FLAGS)

TEMP_DIR := $(shell mktemp -d)

all: all-container

sub-container-%:
	$(MAKE) ARCH=$* container

sub-push-%:
	$(MAKE) ARCH=$* push

all-container: $(addprefix sub-container-,$(ALL_ARCH))

all-push: $(addprefix sub-push-,$(ALL_ARCH))

container: .container-$(ARCH)
.container-$(ARCH): 
	cp -r * $(TEMP_DIR)
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 godep go build ./...
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 godep go build -o $(TEMP_DIR)/rescheduler
	cd $(TEMP_DIR) && sed -i 's|BASEIMAGE|$(BASEIMAGE)|g' Dockerfile
	docker build --pull -t ${MULTI_ARCH_IMG}:$(TAG) $(TEMP_DIR)

push: .psuh-$(ARCH)
.push-$(ARCH): .container-$(ARCH)
	gcloud docker -- push $(MULTI_ARCH_IMG):$(TAG)
ifeq ($(ARCH), amd64)
	gcloud docker -- push $(IMAGE):$(TAG)
endif

clean:
	rm -f rescheduler

.PHONY: all deps build test-unit container push clean
