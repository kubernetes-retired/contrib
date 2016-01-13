PACKAGE = "k8s.io/contrib/compare"

all: compare

deps: 
	go get -u github.com/tools/godep

compare: deps compare.go
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 godep go build -a -installsuffix cgo -ldflags "-w" -o compare

test: 
	godep go test ${PACKAGE}

clean:
	rm -f compare