OS=$(shell uname -s)
GOVERSION=1.6
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

TAG=0.1
IMAGE_NAME?=loadbalancer-daemon

ifeq ($(OS),Linux)
	GO_BUILD:=CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o loadbalancer-daemon *.go
	GO_TEST=godep go test ./...
else
	GO_BUILD:=docker run --rm -v $(ROOT_DIR)/../:/go/src/k8s.io/contrib/loadbalancer -w /go/src/k8s.io/contrib/loadbalancer/loadbalancer-daemon golang:$(GOVERSION) make
	GO_TEST:=$(GO_BUILD) test
endif

all: loadbalancer-daemon

godep:
	go get github.com/tools/godep

loadbalancer-daemon: godep
	$(GO_BUILD)

test: godep
	$(GO_TEST)

container: clean loadbalancer-daemon test
	docker build -t $(IMAGE_NAME):$(TAG) .

push: container
	docker push $(IMAGE_NAME):$(TAG)

osx: godep
	CGO_ENABLED=0 GOOS=darwin godep go build -a -installsuffix cgo -ldflags '-w' -o osx-loadbalancer-daemon *.go

clean:
	rm -f loadbalancer-daemon
