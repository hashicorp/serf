package serf

import (
	"reflect"
	"testing"
)

func TestUserEventCoalesce_Basic(t *testing.T) {
	c := newUserEventCoalescer()

	send := []Event{
		UserEvent{
			LTime: 1,
			Name:  "foo",
		},
		UserEvent{
			LTime: 2,
			Name:  "foo",
		},
		UserEvent{
			LTime:   2,
			Name:    "bar",
			Payload: []byte("test1"),
		},
		UserEvent{
			LTime:   2,
			Name:    "bar",
			Payload: []byte("test2"),
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

	var gotFoo, gotBar1, gotBar2 bool
	for i := 0; i < 3; i++ {
		select {
		case e := <-out:
			ue := e.(UserEvent)
			switch ue.Name {
			case "foo":
				if ue.LTime != 2 {
					t.Fatalf("bad ltime for foo %#v", ue)
				}
				gotFoo = true
			case "bar":
				if ue.LTime != 2 {
					t.Fatalf("bad ltime for bar %#v", ue)
				}
				if reflect.DeepEqual(ue.Payload, []byte("test1")) {
					gotBar1 = true
				}
				if reflect.DeepEqual(ue.Payload, []byte("test2")) {
					gotBar2 = true
				}
			default:
				t.Fatalf("Bad msg %#v", ue)
			}

		default:
			t.Fatalf("should have message")
		}
	}

	if !gotFoo || !gotBar1 || !gotBar2 {
		t.Fatalf("missing messages %v %v %v", gotFoo, gotBar1, gotBar2)
	}
}

func TestUserEventCoalesce_passThrough(t *testing.T) {
	c := newUserEventCoalescer()

	send := []Event{
		MemberEvent{},
	}

	for _, e := range send {
		if c.Handle(e) {
			t.Fatalf("unexpected handle: %#v", e)
		}
	}
}
