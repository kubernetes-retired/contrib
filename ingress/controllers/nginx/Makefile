all: push

# 0.0 shouldn't clobber any release builds
RELEASE = 0.8.3
PREFIX = gcr.io/google_containers/nginx-ingress-controller

REPO_INFO=$(shell git config --get remote.origin.url)

ifndef COMMIT
  COMMIT := git-$(shell git rev-parse --short HEAD)
endif

PKG := "k8s.io/contrib/ingress/controllers/nginx"

controller: clean
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix "static" \
		-ldflags "-X ${PKG}/pkg/version.RELEASE=${RELEASE} -X ${PKG}/pkg/version.COMMIT=${COMMIT} -X ${PKG}/pkg/version.REPO=${PKG}" \
		-o nginx-ingress-controller ${PKG}/pkg/cmd/controller 

container: controller
	docker build -t $(PREFIX):$(RELEASE) .

push: container
	gcloud docker push $(PREFIX):$(RELEASE)

clean:
	rm -f nginx-ingress-controller
