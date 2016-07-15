all: adjuster

# See pod.yaml for the version currently running-- bump this ahead before rebuilding!
TAG = 0.1

deps:
	go get -u github.com/tools/godep

adjuster: test-trigger.go test-action.go test-checker.go frequency-adjuster.go deps
	CGO_ENABLED=0 GOOS=linux godep go build -a -installsuffix cgo -ldflags '-w' -o frequency-adjuster .

clean:
	rm -f frequency_adjuster
