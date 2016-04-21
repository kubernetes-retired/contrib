all: push

# 0.0 shouldn't clobber any release builds, current "latest" is 0.1
TAG = 0.2
PREFIX = gcr.io/google_containers/ubuntu-slim
BUILD_IMAGE = ubuntu-build
TAR_FILE = rootfs.tar

container:
	docker build -t $(BUILD_IMAGE) -f Dockerfile.build .
	docker create --name $(BUILD_IMAGE) $(BUILD_IMAGE)
	docker export $(BUILD_IMAGE) > $(TAR_FILE)
	docker rm $(BUILD_IMAGE)
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	docker rmi -f $(PREFIX):$(TAG)
	docker rmi -f $(BUILD_IMAGE)
	rm $(TAR_FILE)
