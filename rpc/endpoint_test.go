package rpc

import (
	"github.com/hashicorp/serf/serf"
	"testing"
)

func testSerf(t *testing.T) *serf.Serf {
	config := serf.DefaultConfig()
	config.MemberlistConfig.BindAddr = getBindAddr().String()

	s, err := serf.Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return s
}

func TestEndpointMembers(t *testing.T) {
	s1 := testSerf(t)
	defer s1.Shutdown()

	e := &endpoint{serf: s1}

	var result []serf.Member
	err := e.Members(nil, &result)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) != 1 {
		t.Fatalf("bad: %d", len(result))
	}
}
