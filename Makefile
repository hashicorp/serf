DEPS = $(go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

deps:
	go get -d -v ./...
	echo $(DEPS) | xargs -n1 go get -d

test: deps subnet
	go list ./... | xargs -n1 go test

integ: subnet
	go list ./... | INTEG_TESTS=yes xargs -n1 go test

subnet:
	./scripts/setup_test_subnet.sh

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PHONY: all cov deps integ subnet test web web-push
