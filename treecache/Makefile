all: binary

BINARY = treecache
binary: treenode.go treecache.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o $(BINARY) ./treecache.go ./treenode.go

clean:
	rm -f $(BINARY)
