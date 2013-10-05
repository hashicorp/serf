package serf

import (
	"reflect"
	"testing"
)

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
