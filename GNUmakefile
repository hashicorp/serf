SHELL := bash

GOTOOLS := github.com/mitchellh/gox
VERSION := $(shell awk -F\" '/Version = "(.*)"/ { print $$2; exit }' version/version.go)
GITSHA:=$(shell git rev-parse HEAD)
GITBRANCH:=$(shell git symbolic-ref --short HEAD 2>/dev/null)

GOFILES ?= $(shell go list ./...)

default: test

# bin generates the releasable binaries
bin: tools
	@sh -c "'$(CURDIR)/scripts/build.sh'"

cov:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

format:
	@echo "--> Running go fmt"
	@go fmt $(GOFILES)

# dev creates binaries for testing locally - these are put into ./bin and
# $GOPATH
dev:
	@SERF_DEV=1 sh -c "'$(CURDIR)/scripts/build.sh'"

# dist creates the binaries for distibution
dist:
	@sh -c "'$(CURDIR)/scripts/dist.sh' $(VERSION)"

get-tools:
	go get -u -v $(GOTOOLS)

# subnet sets up the require subnet for testing on darwin (osx) - you must run
# this before running other tests if you are on osx.
subnet:
	@sh -c "'$(CURDIR)/scripts/setup_test_subnet.sh'"

# test runs the test suite
test: subnet vet
	go test ./...

# testrace runs the race checker
testrace: subnet vet
	go test -race ./... $(TESTARGS)

tools:
	@which gox 2>/dev/null ; if [ $$? -eq 1 ]; then \
		$(MAKE) get-tools; \
	fi

vet:
	@echo "--> Running go vet"
	@go vet -tags '$(GOTAGS)' $(GOFILES); if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

.PHONY: default bin cov format dev dist get-tools subnet test testrace tools vet
