all: push

# 0.0 shouldn't clobber any released builds
TAG = 0.0
PREFIX = bprashanth/haproxyhttps

server:
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o daemonizer ./daemonizer.go

container: server
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	rm -f *.key
	rm -f *.crt
	rm -f *.json
	rm -f daemonizer
