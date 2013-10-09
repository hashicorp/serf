package rpc

import (
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"testing"
	"time"
)

func testSerf(t *testing.T) (*serf.Serf, string) {
	config := serf.DefaultConfig()
	config.MemberlistConfig.BindAddr = testutil.GetBindAddr().String()
	config.NodeName = config.MemberlistConfig.BindAddr

	s, err := serf.Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return s, config.MemberlistConfig.BindAddr
}

func yield() {
	time.Sleep(5 * time.Millisecond)
}
