
FLAGS =
ENVVAR = GOOS=linux GOARCH=amd64 CGO_ENABLED=0
REGISTRY = gcr.io/google-containers
TAG = v0.3.3
ARCH ?= $(shell go env GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le s390x


IMAGE = $(REGISTRY)/rescheduler
MULTI_ARCH_IMG = $(IMAGE)-$(ARCH)

BASEIMAGE ?= busybox:latest

ifeq ($(ARCH),arm)
        BASEIMAGE=arm32v7/busybox:latest
endif
ifeq ($(ARCH),arm64)
        BASEIMAGE=arm64v8/busybox:latest
endif
ifeq ($(ARCH),ppc64le)
        BASEIMAGE=ppc64le/busybox:latest
endif
ifeq ($(ARCH),s390x)
	BASEIMAGE=s390x/busybox:latest
endif

build: clean 
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build ./...
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -o rescheduler

test-unit: clean build
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go test --test.short -race ./... $(FLAGS)

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
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build ./...
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -o $(TEMP_DIR)/rescheduler
	cd $(TEMP_DIR) && sed -i 's|BASEIMAGE|$(BASEIMAGE)|g' Dockerfile
	docker build --pull -t ${MULTI_ARCH_IMG}:$(TAG) $(TEMP_DIR)

push: .push-$(ARCH)
.push-$(ARCH): .container-$(ARCH)
	gcloud docker -- push $(MULTI_ARCH_IMG):$(TAG)
ifeq ($(ARCH),amd64)
	gcloud docker -- push $(IMAGE):$(TAG)
endif

clean:
	rm -f rescheduler

.PHONY: all build test-unit container push clean
