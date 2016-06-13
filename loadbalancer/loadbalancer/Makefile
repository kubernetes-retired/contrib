OS=$(shell uname -s)
GOVERSION=1.6
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

TAG=0.1
IMAGE_NAME?=ingress-loadbalancer

ifeq ($(OS),Linux)
	GO_BUILD:=CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o ingress-loadbalancer *.go
	GO_TEST=godep go test ./...
else
	GO_BUILD:=docker run --rm -v $(ROOT_DIR)/../:/go/src/k8s.io/contrib/loadbalancer -w /go/src/k8s.io/contrib/loadbalancer/loadbalancer golang:$(GOVERSION) make
	GO_TEST:=$(GO_BUILD) test
endif

all: ingress-loadbalancer

godep:
	go get github.com/tools/godep

ingress-loadbalancer: godep
	$(GO_BUILD)

test: godep
	$(GO_TEST)

container: clean ingress-loadbalancer test
	docker build -t $(IMAGE_NAME):$(TAG) .

push: container
	docker push $(IMAGE_NAME):$(TAG)

osx: godep
	CGO_ENABLED=0 GOOS=darwin godep go build -a -installsuffix cgo -ldflags '-w' -o osx-ingress-loadbalancer *.go

clean:
	rm -f ingress-loadbalancer
