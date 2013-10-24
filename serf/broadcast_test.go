package serf

import (
	"testing"
	"time"
)

func TestBroadcastFinished(t *testing.T) {
	t.Parallel()

	ch := make(chan struct{})
	b := &broadcast{notify: ch}
	b.Finished()

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		t.Fatalf("should have notified")
	}
}

func TestBroadcastFinished_nilNotify(t *testing.T) {
	t.Parallel()

	b := &broadcast{notify: nil}
	b.Finished()
}
