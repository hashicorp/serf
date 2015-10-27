TEST?=./...
VERSION = $(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)

default: test

# bin generates the releasable binaries
bin: generate
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
updatedeps:
	go get -u github.com/mitchellh/gox
	go get -f -t -u ./...
	go list ./... \
		| xargs go list -f '{{join .Deps "\n"}}' \
		| grep -v github.com/hashicorp/serf \
		| grep -v '/internal/' \
		| sort -u \
		| xargs go get -f -u

# generate runs `go generate` to build the dynamically generated source files
generate:
	find . -type f -name '.DS_Store' -delete
	go generate ./...

.PHONY: default bin cov dev dist subnet test testrace updatedeps generate
