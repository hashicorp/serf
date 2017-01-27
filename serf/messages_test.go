package serf

import (
	"net"
	"reflect"
	"testing"
)

func TestQueryFlags(t *testing.T) {
	if queryFlagAck != 1 {
		t.Fatalf("Bad: %v", queryFlagAck)
	}
	if queryFlagNoBroadcast != 2 {
		t.Fatalf("Bad: %v", queryFlagNoBroadcast)
	}
}

func TestEncodeMessage(t *testing.T) {
	in := &messageLeave{Node: "foo"}
	raw, err := encodeMessage(messageLeaveType, in)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if raw[0] != byte(messageLeaveType) {
		t.Fatal("should have type header")
	}

	var out messageLeave
	if err := decodeMessage(raw[1:], &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("mis-match")
	}
}

func TestEncodeRelayMessage(t *testing.T) {
	in := &messageLeave{Node: "foo"}
	addr := net.UDPAddr{IP: net.IP{127, 0, 0, 1}, Port: 1234}
	raw, err := encodeRelayMessage(messageLeaveType, addr, in)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if raw[0] != byte(messageRelayType) {
		t.Fatal("should have type header")
	}

	addrLen := int(raw[1])
	if addrLen != len(addr.String()) {
		t.Fatalf("bad: %d, %d", addrLen, len(addr.String()))
	}

	rawAddr, err := net.ResolveUDPAddr("udp", string(raw[2:addrLen+2]))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if rawAddr.IP.String() != addr.IP.String() || rawAddr.Port != addr.Port {
		t.Fatalf("bad: %v, %v", rawAddr, addr)
	}

	if raw[addrLen+2] != byte(messageLeaveType) {
		t.Fatal("should have type header")
	}

	var out messageLeave
	if err := decodeMessage(raw[addrLen+3:], &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("mis-match")
	}
}

func TestEncodeFilter(t *testing.T) {
	nodes := []string{"foo", "bar"}

	raw, err := encodeFilter(filterNodeType, nodes)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if raw[0] != byte(filterNodeType) {
		t.Fatal("should have type header")
	}

	var out []string
	if err := decodeMessage(raw[1:], &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(nodes, out) {
		t.Fatalf("mis-match")
	}
}
