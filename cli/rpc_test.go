package cli

import (
	"flag"
	"testing"
)

func TestRPCAddrFlag(t *testing.T) {
	f := flag.NewFlagSet("test", flag.ContinueOnError)
	addr := RPCAddrFlag(f)
	if err := f.Parse([]string{"-rpc-addr=foo"}); err != nil {
		t.Fatalf("err: %s", err)
	}

	if *addr != "foo" {
		t.Fatalf("bad: %s", *addr)
	}
}
