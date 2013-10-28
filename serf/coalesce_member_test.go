package serf

import (
	"reflect"
	"sort"
	"testing"
)

func TestMemberEventCoalesce_Basic(t *testing.T) {
	c := newMemberEventCoalescer()

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
		if !c.Handle(e) {
			t.Fatalf("Expected event to be handled: %v", e)
		}
		c.Coalesce(e)
	}

	out := make(chan Event, 64)
	c.Flush(out)

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
	default:
		t.Fatalf("should have message")
	}
}

func TestMemberEventCoalesce_passThrough(t *testing.T) {
	c := newMemberEventCoalescer()

	send := []Event{
		UserEvent{
			Name:    "test",
			Payload: []byte("foo"),
		},
	}

	for _, e := range send {
		if c.Handle(e) {
			t.Fatalf("unexpected handle: %#v", e)
		}
	}
}
