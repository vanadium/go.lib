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

pkgs := $(find ./* -type d)
src:
	mkdir -p src/v.io/x
	rsync -a ${pkgs} src/v.io/x
	go get -t v.io/x/lib/...

test-all: test test-integration

.PHONY: test
test:
	go test v.io/...

.PHONY: clean
clean:
	rm -rf go/bin go/pkg
