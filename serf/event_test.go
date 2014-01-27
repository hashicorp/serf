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

// testUserEvents tests that the given sequence of usr events
// on the event channel took place.
func testUserEvents(t *testing.T, ch <-chan Event, expectedName []string, expectedPayload [][]byte) {
	actualName := make([]string, 0, len(expectedName))
	actualPayload := make([][]byte, 0, len(expectedPayload))

TESTEVENTLOOP:
	for {
		select {
		case r, ok := <-ch:
			if !ok {
				break TESTEVENTLOOP
			}
			u, ok := r.(UserEvent)
			if !ok {
				continue
			}

			actualName = append(actualName, u.Name)
			actualPayload = append(actualPayload, u.Payload)
		case <-time.After(10 * time.Millisecond):
			break TESTEVENTLOOP
		}
	}

	if !reflect.DeepEqual(actualName, expectedName) {
		t.Fatalf("expected names: %v. Got: %v", expectedName, actualName)
	}
	if !reflect.DeepEqual(actualPayload, expectedPayload) {
		t.Fatalf("expected payloads: %v. Got: %v", expectedPayload, actualPayload)
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

	me.Type = EventMemberUpdate
	if me.EventType() != EventMemberUpdate {
		t.Fatalf("bad event type")
	}
	if me.String() != "member-update" {
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

func TestEventType_String(t *testing.T) {
	events := []EventType{EventMemberJoin, EventMemberLeave, EventMemberFailed,
		EventMemberUpdate, EventUser}
	expect := []string{"member-join", "member-leave", "member-failed",
		"member-update", "user"}

	for idx, event := range events {
		if event.String() != expect[idx] {
			t.Fatalf("expect %v got %v", expect[idx], event.String())
		}
	}

	other := EventType(100)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	other.String()
}
