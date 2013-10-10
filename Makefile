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
	INTEG_TESTS=yes go test ./...

subnet:
	echo ./test/setup_subnet.sh

website:
	./scripts/website_run.sh

website-push:
	./scripts/website_push.sh

.PNONY: all cov deps integ subnet test website website-push
