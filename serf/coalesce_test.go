package serf

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func testCoalescer(cPeriod, qPeriod time.Duration) (chan<- Event, <-chan Event, chan<- struct{}) {
	if cPeriod == 0 {
		cPeriod = 10 * time.Millisecond
	}

	if qPeriod == 0 {
		qPeriod = 5 * time.Millisecond
	}

	out := make(chan Event)
	shutdown := make(chan struct{})
	in := coalescedEventCh(out, shutdown, cPeriod, qPeriod)
	return in, out, shutdown
}

func TestCoalescer_basic(t *testing.T) {
	in, out, shutdown := testCoalescer(0, time.Second)
	defer close(shutdown)

	send := []Event{
		MemberEvent{
			Type:    EventMemberJoin,
			Members: []Member{Member{Name: "foo"}},
		},
		MemberEvent{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "foo"}},
		},
		MemberEvent{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "bar"}},
		},
	}

	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.EventType() != EventMemberLeave {
			t.Fatalf("expected leave, got: %d", e.EventType())
		}

		if len(e.(MemberEvent).Members) != 2 {
			t.Fatalf("should have two members: %d", len(e.(MemberEvent).Members))
		}

		expected := []string{"bar", "foo"}
		names := []string{e.(MemberEvent).Members[0].Name, e.(MemberEvent).Members[1].Name}
		sort.Strings(names)

		if !reflect.DeepEqual(expected, names) {
			t.Fatalf("bad: %#v", names)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}

func TestCoalescer_quiescent(t *testing.T) {
	// This tests the quiescence by creating a long coalescence period
	// with a short quiescent period and waiting only a multiple of the
	// quiescent period for results.
	in, out, shutdown := testCoalescer(10*time.Second, 10*time.Millisecond)
	defer close(shutdown)

	send := []Event{
		MemberEvent{
			Type:    EventMemberJoin,
			Members: []Member{Member{Name: "foo"}},
		},
		MemberEvent{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "foo"}},
		},
		MemberEvent{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "bar"}},
		},
	}

	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.EventType() != EventMemberLeave {
			t.Fatalf("expected leave, got: %d", e.EventType())
		}

		if len(e.(MemberEvent).Members) != 2 {
			t.Fatalf("should have two members: %d", len(e.(MemberEvent).Members))
		}

		expected := []string{"bar", "foo"}
		names := []string{e.(MemberEvent).Members[0].Name, e.(MemberEvent).Members[1].Name}
		sort.Strings(names)

		if !reflect.DeepEqual(expected, names) {
			t.Fatalf("bad: %#v", names)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}

func TestCoalescer_passThrough(t *testing.T) {
	in, out, shutdown := testCoalescer(time.Second, time.Second)
	defer close(shutdown)

	send := []Event{
		UserEvent{
			Name:    "test",
			Payload: []byte("foo"),
		},
	}

	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.EventType() != EventUser {
			t.Fatalf("expected user event, got: %d", e.EventType())
		}

		if e.(UserEvent).Name != "test" {
			t.Fatalf("name should be test. %v", e)
		}

		if !reflect.DeepEqual([]byte("foo"), e.(UserEvent).Payload) {
			t.Fatalf("bad: %#v", e.(UserEvent).Payload)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}
