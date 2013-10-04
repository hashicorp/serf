package serf

import (
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	msg := &leave{Node: "foo"}
	buf, err := encode(leaveMsg, msg)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	var out leave
	if err := decode(buf.Bytes()[1:], &out); err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	if out.Node != "foo" {
		t.Fatalf("bad node")
	}
}

func TestRandomOffset(t *testing.T) {
	vals := make(map[int]struct{})
	for i := 0; i < 100; i++ {
		offset := randomOffset(2 << 30)
		if _, ok := vals[offset]; ok {
			t.Fatalf("got collision")
		}
		vals[offset] = struct{}{}
	}
}

func TestRandomOffset_Zero(t *testing.T) {
	offset := randomOffset(0)
	if offset != 0 {
		t.Fatalf("bad offset")
	}
}
