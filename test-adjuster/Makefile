all: adjuster

# See pod.yaml for the version currently running-- bump this ahead before rebuilding!
TAG = 0.1

adjuster: adjuster.go
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o adjuster

clean:
	rm -f adjuster
