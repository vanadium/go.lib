SHELL := /bin/bash -euo pipefail

GOPATH ?= $(shell pwd)
export GOPATH

.PHONY: go
go: get-deps
	go build ./...
	go install ./...

.PHONY: get-deps
get-deps:
	find . -type d
	go get -t ./...

.PHONY: test
test:
	go test v.io/...

.PHONY: clean
clean:
	rm -rf go/bin go/pkg
