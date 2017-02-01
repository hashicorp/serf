package serf

import (
	"fmt"
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

// testQueryEvents tests that the given sequence of queries
// on the event channel took place.
func testQueryEvents(t *testing.T, ch <-chan Event, expectedName []string, expectedPayload [][]byte) {
	actualName := make([]string, 0, len(expectedName))
	actualPayload := make([][]byte, 0, len(expectedPayload))

TESTEVENTLOOP:
	for {
		select {
		case r, ok := <-ch:
			if !ok {
				break TESTEVENTLOOP
			}
			q, ok := r.(*Query)
			if !ok {
				continue
			}

			actualName = append(actualName, q.Name)
			actualPayload = append(actualPayload, q.Payload)
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

	me.Type = EventMemberReap
	if me.EventType() != EventMemberReap {
		t.Fatalf("bad event type")
	}
	if me.String() != "member-reap" {
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

func TestQuery(t *testing.T) {
	q := Query{
		LTime:   42,
		Name:    "update",
		Payload: []byte("abcd1234"),
	}
	if q.EventType() != EventQuery {
		t.Fatalf("Bad")
	}
	if q.String() != "query: update" {
		t.Fatalf("bad: %v", q.String())
	}
}

func TestEventType_String(t *testing.T) {
	events := []EventType{EventMemberJoin, EventMemberLeave, EventMemberFailed,
		EventMemberUpdate, EventMemberReap, EventUser, EventQuery}
	expect := []string{"member-join", "member-leave", "member-failed",
		"member-update", "member-reap", "user", "query"}

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

func TestKRandomNodes(t *testing.T) {
	nodes := []Member{}
	for i := 0; i < 90; i++ {
		// Half the nodes are in a bad state
		state := StatusAlive
		switch i % 3 {
		case 0:
			state = StatusAlive
		case 1:
			state = StatusFailed
		case 2:
			state = StatusLeft
		}
		nodes = append(nodes, Member{
			Name:   fmt.Sprintf("test%d", i),
			Status: state,
		})
	}

	filterFunc := func(m Member) bool {
		if m.Name == "test0" || m.Status != StatusAlive {
			return true
		}
		return false
	}

	s1 := kRandomMembers(3, nodes, filterFunc)
	s2 := kRandomMembers(3, nodes, filterFunc)
	s3 := kRandomMembers(3, nodes, filterFunc)

	if reflect.DeepEqual(s1, s2) {
		t.Fatalf("unexpected equal")
	}
	if reflect.DeepEqual(s1, s3) {
		t.Fatalf("unexpected equal")
	}
	if reflect.DeepEqual(s2, s3) {
		t.Fatalf("unexpected equal")
	}

	for _, s := range [][]Member{s1, s2, s3} {
		if len(s) != 3 {
			t.Fatalf("bad len")
		}
		for _, m := range s {
			if m.Name == "test0" {
				t.Fatalf("Bad name")
			}
			if m.Status != StatusAlive {
				t.Fatalf("Bad state")
			}
		}
	}
}
