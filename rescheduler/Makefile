all: build

FLAGS=
ENVVAR=

deps:
	go get github.com/tools/godep

build: clean deps
	$(ENVVAR) godep go build ./...
	$(ENVVAR) godep go build -o rescheduler 

test-unit: clean deps build
	$(ENVVAR) godep go test --test.short -race ./... $(FLAGS)

clean:
	rm -f rescheduler

.PHONY: all deps build test-unit clean
