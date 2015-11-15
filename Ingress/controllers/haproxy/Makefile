all: push

# 0.0 shouldn't clobber any released builds
TAG = 0.0
PREFIX = gcr.io/google_containers/hlbc

server: hlbc.go
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o hlbc ./hlbc.go ./loadbalancer_log.go

container: server
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	# remove hlbc image
	docker rmi -f $(PREFIX):$(TAG) || true
