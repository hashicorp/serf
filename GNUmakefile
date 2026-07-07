MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -euc
.DEFAULT_GOAL := dev

VERSION := $(shell awk -F\" '/Version = "(.*)"/ { print $$2; exit }' version/version.go)
GITSHA:=$(shell git rev-parse HEAD)
GITBRANCH:=$(shell git symbolic-ref --short HEAD 2>/dev/null)

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)
GIT_COMMIT_FLAG = $(GO_MODULE)/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)

GO_MODULE = github.com/hashicorp/serf
GO_LDFLAGS = -X $(GIT_COMMIT_FLAG)
GO_PKGS ?= $(shell go list ./...)
GO_TAGS = "hashicorpmetrics"

# ----------------------------------------------------------
# build and package for distribution

.PHONY: dev dist clean

# dev creates binaries for testing locally - these are put into ./bin
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: DEV_TARGET=pkg/$(GOOS)_$(GOARCH)/serf
dev:
	$(MAKE) --no-print-directory $(DEV_TARGET)
	mkdir -p ./bin
	cp $(DEV_TARGET) ./bin/serf

ALL_TARGETS = linux_amd64 \
	linux_arm64 \
	darwin_arm64 \
	windows_amd64 \
	freebsd_amd64 \
	openbsd_amd64 \
	solaris_amd64 \
	illumos_amd64

# dist creates the binaries for distibution and bundles them into zip files
dist: $(foreach t,$(ALL_TARGETS),pkg/serf_$(t).zip)

pkg/serf_%.zip: pkg/%/serf
	zip pkg/serf_$(VERSION)_$*.zip pkg/$*/*

pkg/%/serf: GO_OUT ?= $@
pkg/%/serf:
	CGO_ENABLED=0 \
		GOOS=$(firstword $(subst _, ,$*)) \
	 	GOARCH=$(lastword $(subst _, ,$*)) \
	 	go build -trimpath -ldflags "$(GO_LDFLAGS)" -tags "$(GO_TAGS)" -o $(GO_OUT) ./cmd/serf

pkg/windows_%/serf: GO_OUT = $@.exe

clean:
	rm -r pkg
	rm -r bin

# ----------------------------------------------------------
# testing

.PHONY: test subnet cov testrace check lint copywriteheaders tidy

# test runs the test suite
test: subnet
	go test ./...

# subnet sets up the require subnet for testing on darwin (osx) - you must run
# this before running other tests if you are on osx.
subnet:
	@sh -c "'$(CURDIR)/scripts/setup_test_subnet.sh'"

# cov runs tests with a coverage profile
cov:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# testrace runs the race checker
testrace: subnet vet
	go test -race ./... $(TESTARGS)

# check runs all the linters and custom checks
check: lint tidy copywriteheaders

# lint covers go vet and go fmt
lint:
	golangci-lint run --build-tags "$(GO_TAGS)"

# make sure our copyright headers are correct
copywriteheaders:
	copywrite headers --plan

# make sure go.mod/sum are up to date
tidy:
	go mod tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
