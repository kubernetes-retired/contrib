all: test
.PHONY: test
test:
	# SQLite requires CGO. It's only used in test.
	CGO_ENABLED=1 GOOS=linux go test ./...
