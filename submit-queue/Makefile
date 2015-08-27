all: push

submit-queue: submit-queue.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' ./submit-queue.go

container: submit-queue
	docker build -t gcr.io/google_containers/submit-queue:0.2 .

push: container
	gcloud docker push gcr.io/google_containers/submit-queue:0.2

clean:
	rm -f submit-queue
