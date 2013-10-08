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
	in, out, shutdown := testCoalescer(0, 0)
	defer close(shutdown)

	send := []Event{
		Event{
			Type:    EventMemberJoin,
			Members: []Member{Member{Name: "foo"}},
		},
		Event{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "foo"}},
		},
		Event{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "bar"}},
		},
	}

	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.Type != EventMemberLeave {
			t.Fatalf("expected leave, got: %d", e.Type)
		}

		if len(e.Members) != 2 {
			t.Fatalf("should have two members: %d", len(e.Members))
		}

		expected := []string{"bar", "foo"}
		names := []string{e.Members[0].Name, e.Members[1].Name}
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
		Event{
			Type:    EventMemberJoin,
			Members: []Member{Member{Name: "foo"}},
		},
		Event{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "foo"}},
		},
		Event{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "bar"}},
		},
	}

	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.Type != EventMemberLeave {
			t.Fatalf("expected leave, got: %d", e.Type)
		}

		if len(e.Members) != 2 {
			t.Fatalf("should have two members: %d", len(e.Members))
		}

		expected := []string{"bar", "foo"}
		names := []string{e.Members[0].Name, e.Members[1].Name}
		sort.Strings(names)

		if !reflect.DeepEqual(expected, names) {
			t.Fatalf("bad: %#v", names)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}
