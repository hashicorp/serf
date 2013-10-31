package agent

import (
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"strings"
	"testing"
	"time"
)

func TestAgent_eventHandler(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	handler := new(MockEventHandler)
	a1.EventHandler = handler

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if len(handler.Events) != 1 {
		t.Fatalf("bad: %#v", handler.Events)
	}

	if handler.Events[0].EventType() != serf.EventMemberJoin {
		t.Fatalf("bad: %#v", handler.Events[0])
	}
}

func TestAgent_events(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	eventsCh := make(chan string, 64)
	prev := a1.NotifyLogs(eventsCh)
	defer a1.StopLogs(eventsCh)

	if len(prev) < 1 {
		t.Fatalf("bad: %d", len(prev))
	}

	a1.Join(nil, false)

	select {
	case e := <-eventsCh:
		if !strings.Contains(e, "join") {
			t.Fatalf("bad: %s", e)
		}
	case <-time.After(5 * time.Millisecond):
		t.Fatal("timeout")
	}
}

func TestAgentShutdown_multiple(t *testing.T) {
	a := testAgent()
	if err := a.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	for i := 0; i < 5; i++ {
		if err := a.Shutdown(); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
}

func TestAgentShutdown_noStart(t *testing.T) {
	a := testAgent()
	if err := a.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestAgentUserEvent(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	handler := new(MockEventHandler)
	a1.EventHandler = handler

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := a1.UserEvent("deploy", []byte("foo"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	handler.Lock()
	defer handler.Unlock()

	if len(handler.Events) == 0 {
		t.Fatal("no events")
	}

	e, ok := handler.Events[len(handler.Events)-1].(serf.UserEvent)
	if !ok {
		t.Fatalf("bad: %#v", e)
	}

	if e.Name != "deploy" {
		t.Fatalf("bad: %#v", e)
	}

	if string(e.Payload) != "foo" {
		t.Fatalf("bad: %#v", e)
	}
}
