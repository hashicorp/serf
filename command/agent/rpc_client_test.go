package agent

import (
	"bytes"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// testRPCClient returns an RPCClient connected to an RPC server that
// serves only this connection.
func testRPCClient(t *testing.T) (*RPCClient, *Agent, *AgentIPC) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	lw := NewLogWriter(512)
	mult := io.MultiWriter(os.Stderr, lw)

	agent := testAgent(mult)
	ipc := NewAgentIPC(agent, l, mult, lw)

	rpcClient, err := NewRPCClient(l.Addr().String())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return rpcClient, agent, ipc
}

func TestRPCClientForceLeave(t *testing.T) {
	client, a1, ipc := testRPCClient(t)
	a2 := testAgent(nil)
	defer ipc.Shutdown()
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

	s2Addr := a2.conf.MemberlistConfig.BindAddr
	if _, err := a1.Join([]string{s2Addr}, false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := a2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(a1.conf.MemberlistConfig.ProbeInterval * 7)

	if err := client.ForceLeave(a2.conf.NodeName); err != nil {
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
	client, a1, ipc := testRPCClient(t)
	a2 := testAgent(nil)
	defer ipc.Shutdown()
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

	n, err := client.Join([]string{a2.conf.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("n != 1: %d", n)
	}
}

func TestRPCClientMembers(t *testing.T) {
	client, a1, ipc := testRPCClient(t)
	a2 := testAgent(nil)
	defer ipc.Shutdown()
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

	_, err = client.Join([]string{a2.conf.MemberlistConfig.BindAddr}, false)
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
	client, a1, ipc := testRPCClient(t)
	defer ipc.Shutdown()
	defer client.Close()
	defer a1.Shutdown()

	handler := new(MockEventHandler)
	a1.RegisterEventHandler(handler)

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

func TestRPCClientLeave(t *testing.T) {
	client, a1, ipc := testRPCClient(t)
	defer ipc.Shutdown()
	defer client.Close()
	defer a1.Shutdown()

	testutil.Yield()

	if err := client.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	select {
	case <-a1.ShutdownCh():
	default:
		t.Fatalf("agent should be shutdown!")
	}
}

func TestRPCClientMonitor(t *testing.T) {
	client, a1, ipc := testRPCClient(t)
	defer ipc.Shutdown()
	defer client.Close()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	eventCh := make(chan string, 64)
	if handle, err := client.Monitor("debug", eventCh); err != nil {
		t.Fatalf("err: %s", err)
	} else {
		defer client.Stop(handle)
	}

	testutil.Yield()

	select {
	case e := <-eventCh:
		if !strings.Contains(e, "Accepted client") {
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
}

func TestRPCClientStream_User(t *testing.T) {
	client, a1, ipc := testRPCClient(t)
	defer ipc.Shutdown()
	defer client.Close()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	eventCh := make(chan map[string]interface{}, 64)
	if handle, err := client.Stream("user", eventCh); err != nil {
		t.Fatalf("err: %s", err)
	} else {
		defer client.Stop(handle)
	}

	testutil.Yield()

	if err := client.UserEvent("deploy", []byte("foo"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	select {
	case e := <-eventCh:
		if e["Event"].(string) != "user" {
			t.Fatalf("bad event: %#v", e)
		}
		if e["LTime"].(uint64) != 1 {
			t.Fatalf("bad event: %#v", e)
		}
		if e["Name"].(string) != "deploy" {
			t.Fatalf("bad event: %#v", e)
		}
		if bytes.Compare(e["Payload"].([]byte), []byte("foo")) != 0 {
			t.Fatalf("bad event: %#v", e)
		}
		if e["Coalesce"].(bool) != false {
			t.Fatalf("bad event: %#v", e)
		}

	default:
		t.Fatalf("should have event")
	}
}

func TestRPCClientStream_Member(t *testing.T) {
	client, a1, ipc := testRPCClient(t)
	defer ipc.Shutdown()
	defer client.Close()
	defer a1.Shutdown()
	a2 := testAgent(nil)
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	eventCh := make(chan map[string]interface{}, 64)
	if handle, err := client.Stream("*", eventCh); err != nil {
		t.Fatalf("err: %s", err)
	} else {
		defer client.Stop(handle)
	}

	testutil.Yield()

	s2Addr := a2.conf.MemberlistConfig.BindAddr
	if _, err := a1.Join([]string{s2Addr}, false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	select {
	case e := <-eventCh:
		if e["Event"].(string) != "member-join" {
			t.Fatalf("bad event: %#v", e)
		}

		members := e["Members"].([]interface{})
		if len(members) != 1 {
			t.Fatalf("should have 1 member")
		}
		member := members[0].(map[interface{}]interface{})

		if _, ok := member["Name"].(string); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["Addr"].([]interface{}); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["Port"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["Role"].(string); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if stat, _ := member["Status"].(string); stat != "alive" {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["ProtocolMin"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["ProtocolMax"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["ProtocolCur"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["DelegateMin"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["DelegateMax"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}
		if _, ok := member["DelegateCur"].(uint64); !ok {
			t.Fatalf("bad event: %#v", e)
		}

	default:
		t.Fatalf("should have event")
	}
}
