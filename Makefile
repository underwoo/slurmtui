BINARY ?= slurmtui
VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: build test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/slurmtui

test:
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run
	golangci-lint fmt --diff

clean:
	rm -f $(BINARY)
