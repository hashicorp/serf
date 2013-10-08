package serf

import (
	"testing"
)

func TestEncodeMessage(t *testing.T) {
	raw, err := encodeMessage(messageLeaveType, &messageLeave{Node: "foo"})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if raw[0] != byte(messageLeaveType) {
		t.Fatal("should have type header")
	}
}
