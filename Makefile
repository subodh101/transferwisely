# Makefile for transferwisely

.PHONY: all fmt lint test precommit

all: fmt lint test

fmt:
	gofmt -w .

lint:
	golint ./...

test:
	go test ./...

precommit: fmt
