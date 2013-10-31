package agent

import (
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"net"
	"net/rpc"
	"strings"
	"testing"
)

// testRPCClient returns an RPCClient connected to an RPC server that
// serves only this connection.
func testRPCClient(t *testing.T) (*RPCClient, *Agent) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	agent := testAgent()
	server := rpc.NewServer()
	if err := registerEndpoint(server, agent); err != nil {
		l.Close()
		t.Fatalf("err: %s", err)
	}

	go func() {
		conn, err := l.Accept()
		l.Close()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		defer conn.Close()
		server.ServeConn(conn)
	}()

	rpcClient, err := rpc.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return &RPCClient{Client: rpcClient}, agent
}

func TestRPCClientForceLeave(t *testing.T) {
	client, a1 := testRPCClient(t)
	a2 := testAgent()
	defer client.Close()
	defer a1.Shutdown()
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	s2Addr := a2.SerfConfig.MemberlistConfig.BindAddr
	if _, err := a1.Join([]string{s2Addr}, false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := a2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := client.ForceLeave(a2.SerfConfig.NodeName); err != nil {
		t.Fatalf("err: %s", err)
	}

	m := a1.Serf().Members()
	if len(m) != 2 {
		t.Fatalf("should have 2 members: %#v", a1.Serf().Members())
	}

	if m[1].Status != serf.StatusLeft {
		t.Fatalf("should be left: %#v", m[1])
	}
}

func TestRPCClientJoin(t *testing.T) {
	client, a1 := testRPCClient(t)
	a2 := testAgent()
	defer client.Close()
	defer a1.Shutdown()
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	n, err := client.Join([]string{a2.SerfConfig.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("n != 1: %d", n)
	}
}

func TestRPCClientMembers(t *testing.T) {
	client, a1 := testRPCClient(t)
	a2 := testAgent()
	defer client.Close()
	defer a1.Shutdown()
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	mem, err := client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 1 {
		t.Fatalf("bad: %#v", mem)
	}

	_, err = client.Join([]string{a2.SerfConfig.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	mem, err = client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 2 {
		t.Fatalf("bad: %#v", mem)
	}
}

func TestRPCClientUserEvent(t *testing.T) {
	client, a1 := testRPCClient(t)
	defer client.Close()
	defer a1.Shutdown()

	handler := new(MockEventHandler)
	a1.EventHandler = handler

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := client.UserEvent("deploy", []byte("foo"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	handler.Lock()
	defer handler.Unlock()

	if len(handler.Events) == 0 {
		t.Fatal("no events")
	}

	serfEvent, ok := handler.Events[len(handler.Events)-1].(serf.UserEvent)
	if !ok {
		t.Fatalf("bad: %#v", serfEvent)
	}

	if serfEvent.Name != "deploy" {
		t.Fatalf("bad: %#v", serfEvent)
	}

	if string(serfEvent.Payload) != "foo" {
		t.Fatalf("bad: %#v", serfEvent)
	}
}
func TestRPCClientMonitor(t *testing.T) {
	client, a1 := testRPCClient(t)
	defer client.Close()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	eventCh := make(chan string, 64)
	doneCh := make(chan struct{}, 64)
	defer close(doneCh)
	if err := client.Monitor("debug", eventCh, doneCh); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	select {
	case e := <-eventCh:
		if !strings.Contains(e, "starting") {
			t.Fatalf("bad: %s", e)
		}
	default:
		t.Fatalf("should have backlog")
	}

	// Drain the rest of the messages as we know it
	drainEventCh(eventCh)

	// Join a bad thing to generate more events
	a1.Join(nil, false)

	testutil.Yield()

	select {
	case e := <-eventCh:
		if !strings.Contains(e, "joining") {
			t.Fatalf("bad: %s", e)
		}
	default:
		t.Fatalf("should have message")
	}

	// End the monitor and wait for the eventCh to close
	doneCh <- struct{}{}
	for _ = range eventCh {
	}
}
