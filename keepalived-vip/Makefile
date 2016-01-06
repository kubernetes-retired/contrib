all: push

# 0.0 shouldn't clobber any release builds
TAG = 0.0
PREFIX = gcr.io/google_containers/kube-keepalived-vip
BUILD_IMAGE = build-keepalived
APK_NAME = keepalived
APK_VERSION = 1.2.19-r0

controller: clean
	CGO_ENABLED=0 GOOS=linux godep go build -a -ldflags '-w' -o kube-keepalived-vip

container: controller keepalived
	docker build -t $(PREFIX):$(TAG) .

keepalived:
	docker build -t $(BUILD_IMAGE):$(TAG) build
	docker create --name $(BUILD_IMAGE) $(BUILD_IMAGE):$(TAG) true
	# docker cp semantics changed between 1.7 and 1.8, so we cp the file to cwd and rename it.
	docker cp $(BUILD_IMAGE):/work/x86_64/$(APK_NAME)-$(APK_VERSION).apk .
	docker rm -f $(BUILD_IMAGE)
	mv $(APK_NAME)-$(APK_VERSION).apk $(APK_NAME).apk

push: container
	docker push $(PREFIX):$(TAG)

clean:
	rm -f kube-keepalived-vip
