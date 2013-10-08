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
		case e := <-ch:
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
