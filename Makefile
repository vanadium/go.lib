SHELL := /bin/bash -euo pipefail

GOPATH ?= $(shell pwd)
export GOPATH

.PHONY: all
all: go

.PHONY: go
go: get-deps
	go version
	go list v.io/...

.PHONY: get-deps
get-deps: src

src:
	mkdir -p src/v.io
	rsync -a x src/v.io
	go get -t v.io/...

test-all: test test-integration

.PHONY: test
test:
	go test v.io/...

.PHONY: clean
clean:
	rm -rf go/bin go/pkg
