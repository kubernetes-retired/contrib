all: push

# 0.0 shouldn't clobber any release builds
TAG = 0.5
PREFIX = gcr.io/google_containers/dns-controller

REPO_INFO=$(shell git config --get remote.origin.url)

ifndef VERSION
  VERSION := git-$(shell git rev-parse --short HEAD)
endif

#dns-controller: clean
#	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags \
#		"-w -X main.version=${VERSION} -X main.gitRepo=${REPO_INFO}" \
#		-o dns-controller
dns-controller: clean
	CGO_ENABLED=0 GOOS=linux godep go build \
		-ldflags "-w -X main.version=${VERSION} -X main.gitRepo=${REPO_INFO}" \
		-o dns-controller \
		k8s.io/contrib/dns/cmd/dns-controller

container: dns-controller
  docker build -t $(PREFIX):$(TAG) .

push: container
	gcloud docker push $(PREFIX):$(TAG)

clean:
	rm -f dns-controller
