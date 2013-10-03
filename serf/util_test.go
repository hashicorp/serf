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
