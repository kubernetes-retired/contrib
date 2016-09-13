VERSION=v1.0
IMAGE=723255503624.dkr.ecr.us-east-1.amazonaws.com/cluster-autoscaler

clean:
		rm -f cluster-autoscaler

compile: clean
		GOOS=linux go build -a -o cluster-autoscaler

build: compile
		docker build -t $(IMAGE):$(VERSION) .

push: build
		docker push $(IMAGE):$(VERSION)

.PHONY: clean compile build push
