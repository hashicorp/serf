package serf

import (
	"github.com/hashicorp/memberlist"
	"reflect"
	"testing"
	"time"
)

func TestSerf_EventHandler(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	go func() {
		time.Sleep(10 * time.Millisecond)
		t.Fatalf("timeout")
	}()

	n1 := &memberlist.Node{Name: "test", Addr: []byte{127, 0, 0, 1}, Meta: []byte("foo")}
	n2 := &memberlist.Node{Name: "foo", Addr: []byte{127, 0, 0, 2}, Meta: []byte("foo")}

	go func() {
		s.joinCh <- n1
		time.Sleep(time.Microsecond)
		s.joinCh <- n2
		time.Sleep(time.Microsecond)
		s.leaveCh <- n2
		time.Sleep(time.Microsecond)
		close(s.shutdownCh)
	}()
	s.eventHandler()

	if s.members["test"].Status != StatusAlive {
		t.Fatalf("expected test to be alive")
	}
	if s.members["foo"].Status != StatusFailed {
		t.Fatalf("expected foo to be failed: %v", s.members["foo"])
	}
}

func TestSerf_NodeMeta(t *testing.T) {
	c := &Config{Role: "test"}
	s := newSerf(c)

	meta := s.NodeMeta(3)
	if !reflect.DeepEqual(meta, []byte("tes")) {
		t.Fatalf("bad meta data")
	}
}

func TestSerf_NotifyMsg_Leave(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	s.members["test"] = &Member{
		Name:   "test",
		Addr:   []byte{127, 0, 0, 1},
		Role:   "foo",
		Status: StatusAlive,
	}

	l := leave{Node: "test"}
	buf, _ := encode(leaveMsg, &l)
	s.NotifyMsg(buf.Bytes())

	mem := s.members["test"]
	if mem.Status != StatusLeaving {
		t.Fatalf("bad member: %v", *mem)
	}

	if s.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected queued rebroadcast")
	}
}

func TestSerf_NotifyMsg_Empty(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	s.NotifyMsg(nil)
}
