all: push

# See pod.yaml for the version currently running-- bump this ahead before rebuilding!
TAG = v0.3

REPO = staging-k8s.gcr.io

node-perf-dash: node-perf-dash.go parser.go downloader.go types.go kubelet-parser.go
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o node-perf-dash

container: node-perf-dash
	docker build --pull -t $(REPO)/node-perf-dash:$(TAG) .

push: container
	gcloud docker -- push $(REPO)/node-perf-dash:$(TAG)

clean:
	rm -f node-perf-dash
