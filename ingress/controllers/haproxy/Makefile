SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')
MAIN=./cmd/main.go

RELEASE_DIR=./release
IMAGE_DIR=./image
BINARY=haddock
GO_PACKAGE := k8s.io/contrib/ingress/controllers/haproxy/cmd



IMAGE_REPO=quay.io/stackpoint/haproxy-ingress-controller

VERSION := $(shell cat VERSION)
DATE= $(shell date +%FT%T)
GIT_VERSION= $(shell git describe --tags --long --dirty --always)

LDFLAG_VER := -X $(GO_PACKAGE)/model.version=$(VERSION)
LDFLAG_DATE := -X $(GO_PACKAGE)/model.date=$(DATE)
LDFLAG_GIT := -X $(GO_PACKAGE)/model.gitVersion=$(GIT_VERSION)
LDFLAG_STATIC :=-extldflags "-static"

TARGET_OS = darwin linux
TARGET_ARCHS = amd64 386

.DEFAULT_GOAL := cross
.PHONY: clean linux darwin cross deps image test

deps:
	@echo "+ $@"
	which glide;
	glide install -v -s;

linux: $(SOURCES) VERSION
	@echo "+ $@"
	$(call build,linux,amd64)

darwin: $(SOURCES) VERSION
	@echo "+ $@"
	$(call build,darwin,amd64)

cross: $(SOURCES) VERSION
	@echo "+ $@"
	$(foreach OS,$(TARGET_OS),$(foreach ARCH,$(TARGET_ARCHS),$(call build,$(OS),$(ARCH))))


define build
	mkdir -p ${RELEASE_DIR}/$(1)/$(2);
	GOOS=$(1) GOARCH=$(2) go build --ldflags '$(LDFLAG_VER) $(LDFLAG_DATE) $(LDFLAG_GIT) $(LDFLAG_STATIC)' -o ${RELEASE_DIR}/$(1)/$(2)/${BINARY} $(MAIN);
endef

test: $(SOURCES)
	@echo "+ $@"
	go test $(shell go list ./pkg/... | grep -v /vendor/) -v

image: linux
	@echo "+ $@"
	mkdir -p ${IMAGE_DIR}/bin;
	mv ${RELEASE_DIR}/linux/amd64/${BINARY} ${IMAGE_DIR}/bin/${BINARY}

	docker build -t  ${IMAGE_REPO}:${VERSION} ${IMAGE_DIR}
	docker push ${IMAGE_REPO}:${VERSION}

clean:
	@echo "+ $@"
	rm -rf ${RELEASE_DIR}
	rm -f ${IMAGE_DIR}/bin/${BINARY}
