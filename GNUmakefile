GOTOOLS = github.com/mitchellh/gox golang.org/x/tools/cmd/vet github.com/kardianos/govendor
TEST?=./...
VERSION = $(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)
GITSHA:=$(shell git rev-parse HEAD)
GITBRANCH:=$(shell git symbolic-ref --short HEAD 2>/dev/null)

default: test

# bin generates the releasable binaries
bin: generate tools
	@sh -c "'$(CURDIR)/scripts/build.sh'"

# cov generates the coverage output
cov: generate
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

# dev creates binaries for testing locally - these are put into ./bin and
# $GOPATH
dev: generate
	@SERF_DEV=1 sh -c "'$(CURDIR)/scripts/build.sh'"

# dist creates the binaries for distibution
dist: #bin
	@sh -c "'$(CURDIR)/scripts/dist.sh' $(VERSION)"

# subnet sets up the require subnet for testing on darwin (osx) - you must run
# this before running other tests if you are on osx.
subnet:
	@sh -c "'$(CURDIR)/scripts/setup_test_subnet.sh'"

# test runs the test suite
test: subnet generate
	go list $(TEST) | xargs -n1 go test $(TESTARGS)

# testrace runs the race checker
testrace: subnet generate
	go test -race $(TEST) $(TESTARGS)

# updatedeps installs all the dependencies needed to test, build, and run
updatedeps:: tools
	govendor list -no-status +vendor | xargs -n1 go get -u
	govendor update +vendor

# generate runs `go generate` to build the dynamically generated source files
generate:
	find . -type f -name '.DS_Store' -delete
	go list ./... | \
		grep -v ^github.com/hashicorp/serf/vendor | \
		xargs -n1 \
			go generate

vet:
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@echo "--> Running go tool vet $(VETARGS) ."
	@go list ./... \
		| grep -v ^github.com/hashicorp/serf/vendor/ \
		| cut -d '/' -f 4- \
		| xargs -n1 \
			go tool vet $(VETARGS) ;\
	if [ $$? -ne 0 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for reviewal."; \
	fi

tools:
	go get -u -v $(GOTOOLS)

.PHONY: default bin cov dev dist subnet test testrace generate tools vet
