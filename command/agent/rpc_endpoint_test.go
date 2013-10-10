package agent

import (
	"encoding/gob"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"net"
	"strings"
	"testing"
)

func TestRPCEndpointJoin(t *testing.T) {
	a1 := testAgent()
	a2 := testAgent()
	defer a1.Shutdown()
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	e := &rpcEndpoint{agent: a1}

	var n int
	s2Addr := a2.SerfConfig.MemberlistConfig.BindAddr
	err := e.Join([]string{s2Addr}, &n)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("bad n: %d", n)
	}

	testutil.Yield()

	if len(a2.Serf().Members()) != 2 {
		t.Fatalf("should have 2 members: %#v", a2.Serf().Members())
	}
}

func TestRPCEndpointMembers(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	e := &rpcEndpoint{agent: a1}

	var result []serf.Member
	err := e.Members(nil, &result)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) != 1 {
		t.Fatalf("bad: %d", len(result))
	}
}

func TestRPCEndpointMonitor(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer l.Close()

	e := &rpcEndpoint{agent: a1}
	err = e.Monitor(l.Addr().String(), new(interface{}))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	conn, err := l.Accept()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer conn.Close()

	var message string
	dec := gob.NewDecoder(conn)
	if err := dec.Decode(&message); err != nil {
		t.Fatalf("err: %s", err)
	}

	if !strings.Contains(message, "starting") {
		t.Fatalf("bad: %s", message)
	}

	// TODO(mitchellh): test the streaming
}
