all: push

# 0.0 shouldn't clobber any release builds, current "latest" is 0.2
TAG = 0.0
PREFIX = gcr.io/google_containers/kube-keepalived-vip
BUILD_IMAGE = build-keepalived

controller: clean
	CGO_ENABLED=0 GOOS=linux godep go build -a -ldflags '-w' -o kube-keepalived-vip

container: controller
	docker build -t $(PREFIX):$(TAG) .

push: container
	gcloud docker push $(PREFIX):$(TAG)

clean:
	rm -f kube-keepalived-vip
