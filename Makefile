.PHONY: build install test clean run

BINARY=bd-drain
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/bd-drain

install:
	go install $(LDFLAGS) ./cmd/bd-drain

test:
	go test -v ./...

clean:
	rm -f $(BINARY)
	rm -rf .bd-drain/

run: build
	./$(BINARY) start

# Development helpers
fmt:
	go fmt ./...

lint:
	golangci-lint run

deps:
	go mod tidy
	go mod download
