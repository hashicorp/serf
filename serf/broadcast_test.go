package serf

import (
	"github.com/hashicorp/memberlist"
	"testing"
	"time"
)

func TestBroadcast_impl(t *testing.T) {
	t.Parallel()

	var raw interface{}
	raw = new(broadcast)
	if _, ok := raw.(memberlist.Broadcast); !ok {
		t.Fatalf("should be a Broadcast")
	}
}

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
