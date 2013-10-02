test: subnet
	go test ./...

integ: subnet
	INTEG_TESTS=yes go test ./...

subnet:
	./test/setup_subnet.sh

cov:
	gocov test github.com/hashicorp/serf | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

.PNONY: test cov integ
