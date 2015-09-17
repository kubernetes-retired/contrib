all: push

# See pod.yaml for the version currently running-- bump this ahead before rebuilding!
TAG = 0.11

submit-queue: submit-queue.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o submit-queue

container: submit-queue
	docker build -t gcr.io/google_containers/submit-queue:$(TAG) .

push: container
	gcloud docker push gcr.io/google_containers/submit-queue:$(TAG)

clean:
	rm -f submit-queue
