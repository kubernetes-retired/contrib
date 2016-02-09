all: push

# See pod.yaml for the version currently running-- bump this ahead before rebuilding!
TAG = 0.3

perfdash: perfdash.go
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o perfdash

container: perfdash
	docker build -t gcr.io/google_containers/perfdash:$(TAG) .

push: container
	gcloud docker push gcr.io/google_containers/perfdash:$(TAG)

clean:
	rm -f perfdash
