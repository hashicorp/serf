package rpc

import (
	"github.com/hashicorp/serf/serf"
	"testing"
)

func testSerf(t *testing.T) (*serf.Serf, string) {
	config := serf.DefaultConfig()
	config.MemberlistConfig.BindAddr = getBindAddr().String()
	config.NodeName = config.MemberlistConfig.BindAddr

	s, err := serf.Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return s, config.MemberlistConfig.BindAddr
}

func TestEndpointJoin(t *testing.T) {
	s1, _ := testSerf(t)
	s2, s2Addr := testSerf(t)
	defer s1.Shutdown()
	defer s2.Shutdown()

	e := &endpoint{serf: s1}

	var n int
	err := e.Join([]string{s2Addr}, &n)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("bad n: %d", n)
	}

	yield()

	if len(s2.Members()) != 2 {
		t.Fatalf("should have 2 members: %#v", s2.Members())
	}
}

func TestEndpointMembers(t *testing.T) {
	s1, _ := testSerf(t)
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
