package agent

import (
	"encoding/gob"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"log"
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
	err = e.Monitor(RPCMonitorArgs{
		CallbackAddr: l.Addr().String(),
	}, new(interface{}))
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	conn, err := l.Accept()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer conn.Close()

	eventCh := make(chan string, 64)
	go func() {
		var message string

		dec := gob.NewDecoder(conn)
		for {
			if err := dec.Decode(&message); err != nil {
				log.Printf("[TEST] Error decoding message: %s", err)
				return
			}

			eventCh <- message
		}
	}()

	testutil.Yield()

	select {
	case e := <-eventCh:
		if !strings.Contains(e, "starting") {
			t.Fatalf("bad: %s", e)
		}
	default:
		t.Fatalf("should have message right away")
	}

	// Drain the eventCh
DRAIN:
	for {
		select {
		case <-eventCh:
		default:
			break DRAIN
		}
	}

	// Do a join to trigger more log messages. It should stream it.
	a1.Join(nil)

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

func TestRPCEndpointMonitor_badLogLevel(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	e := &rpcEndpoint{agent: a1}
	err := e.Monitor(RPCMonitorArgs{
		CallbackAddr: "",
		LogLevel:     "foo",
	}, new(interface{}))
	if err == nil {
		t.Fatal("should have error")
	}
}

func TestRPCEndpointUserEvent(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	handler := new(MockEventHandler)
	a1.EventHandler = handler

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	e := &rpcEndpoint{agent: a1}
	args := RPCUserEventArgs{
		Name:    "deploy",
		Payload: []byte("foo"),
	}
	if err := e.UserEvent(args, new(interface{})); err != nil {
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
