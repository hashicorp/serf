package serf

import (
	"reflect"
	"testing"
	"time"
)

// testEvents tests that the given node had the given sequence of events
// on the event channel.
func testEvents(t *testing.T, ch <-chan Event, node string, expected []EventType) {
	actual := make([]EventType, 0, len(expected))

TESTEVENTLOOP:
	for {
		select {
		case r := <-ch:
			e, ok := r.(MemberEvent)
			if !ok {
				continue
			}

			found := false
			for _, m := range e.Members {
				if m.Name == node {
					found = true
					break
				}
			}

			if found {
				actual = append(actual, e.Type)
			}
		case <-time.After(10 * time.Millisecond):
			break TESTEVENTLOOP
		}
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected events: %v. Got: %v", expected, actual)
	}
}

func TestMemberEvent(t *testing.T) {
	me := MemberEvent{
		Type:    EventMemberJoin,
		Members: nil,
	}
	if me.EventType() != EventMemberJoin {
		t.Fatalf("bad event type")
	}
	if me.String() != "member-join" {
		t.Fatalf("bad string val")
	}

	me.Type = EventMemberLeave
	if me.EventType() != EventMemberLeave {
		t.Fatalf("bad event type")
	}
	if me.String() != "member-leave" {
		t.Fatalf("bad string val")
	}

	me.Type = EventMemberFailed
	if me.EventType() != EventMemberFailed {
		t.Fatalf("bad event type")
	}
	if me.String() != "member-failed" {
		t.Fatalf("bad string val")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	me.Type = EventUser
	me.String()
}

func TestUserEvent(t *testing.T) {
	ue := UserEvent{
		Name:    "test",
		Payload: []byte("foobar"),
	}
	if ue.EventType() != EventUser {
		t.Fatalf("bad event type")
	}
	if ue.String() != "user-event: test" {
		t.Fatalf("bad string val")
	}
}
