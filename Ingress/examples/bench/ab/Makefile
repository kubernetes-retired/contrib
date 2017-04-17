all: push

TAG = 1.0
PREFIX = bprashanth/nginxtest

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)
