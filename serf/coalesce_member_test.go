package serf

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestMemberEventCoalesce_Basic(t *testing.T) {
	outCh := make(chan Event, 64)
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &memberEventCoalescer{
		lastEvents:   make(map[string]EventType),
		latestEvents: make(map[string]coalesceEvent),
	}

	inCh := coalescedEventCh(outCh, shutdownCh,
		5*time.Millisecond, 5*time.Millisecond, c)

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
		inCh <- e
	}

	events := make(map[EventType]Event)
	timeout := time.After(10 * time.Millisecond)

MEMBEREVENTFORLOOP:
	for {
		select {
		case e := <-outCh:
			events[e.EventType()] = e
		case <-timeout:
			break MEMBEREVENTFORLOOP
		}
	}

	if len(events) != 1 {
		t.Fatalf("bad: %#v", events)
	}

	if e, ok := events[EventMemberLeave]; !ok {
		t.Fatalf("bad: %#v", events)
	} else {
		me := e.(MemberEvent)

		if len(me.Members) != 2 {
			t.Fatalf("bad: %#v", me)
		}

		expected := []string{"bar", "foo"}
		names := []string{me.Members[0].Name, me.Members[1].Name}
		sort.Strings(names)

		if !reflect.DeepEqual(expected, names) {
			t.Fatalf("bad: %#v", names)
		}
	}
}

func TestMemberEventCoalesce_passThrough(t *testing.T) {
	cases := []struct {
		e      Event
		handle bool
	}{
		{UserEvent{}, false},
		{MemberEvent{Type: EventMemberJoin}, true},
		{MemberEvent{Type: EventMemberLeave}, true},
		{MemberEvent{Type: EventMemberFailed}, true},
	}

	for _, tc := range cases {
		c := &memberEventCoalescer{}
		if tc.handle != c.Handle(tc.e) {
			t.Fatalf("bad: %#v", tc.e)
		}
	}
}
