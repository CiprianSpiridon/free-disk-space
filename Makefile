# freedisk — macOS disk-audit CLI
#
# Releases stamp Version with:
#   -X github.com/CiprianSpiridon/free-disk-space/internal/version.Version={{.Version}}
# `make build` does the same from `git describe` (v prefix stripped) or 0.1.0.

MODULE  := github.com/CiprianSpiridon/free-disk-space
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0)
VERSION_NOPREFIX := $(patsubst v%,%,$(VERSION))
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION_NOPREFIX)

.PHONY: build test clean

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/freedisk ./cmd/freedisk

test:
	go test ./...

clean:
	rm -rf bin
