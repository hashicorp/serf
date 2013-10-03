package serf

import (
	"testing"
)

func TestSerfBroadcast(t *testing.T) {
	sb := &serfBroadcast{msg: []byte("test"), notify: make(chan struct{}, 1)}
	if sb.Invalidates(nil) {
		t.Fatalf("unexpected invalidates")
	}
	if len(sb.Message()) != 4 {
		t.Fatalf("bad msg length")
	}
	sb.Finished()
	if len(sb.notify) != 1 {
		t.Fatalf("Expect notify msg")
	}
}

func TestSerf_EncodeBroadcastNotify(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	l := leave{"foo"}
	if err := s.encodeBroadcastNotify(leaveMsg, &l, nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if s.broadcasts.NumQueued() != 1 {
		t.Fatalf("expect queued message")
	}
}

func TestSerf_Rebroadcast(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	s.rebroadcast([]byte("test"))
	if s.broadcasts.NumQueued() != 1 {
		t.Fatalf("expect queued message")
	}
}
