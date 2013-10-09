package serf

import (
	"github.com/hashicorp/memberlist"
	"reflect"
	"testing"
)

func TestDelegate_impl(t *testing.T) {
	var raw interface{}
	raw = new(delegate)
	if _, ok := raw.(memberlist.Delegate); !ok {
		t.Fatal("should be an Delegate")
	}
}

func TestDelegate_NodeMeta(t *testing.T) {
	c := testConfig()
	c.Role = "test"
	d := &delegate{&Serf{config: c}}
	meta := d.NodeMeta(32)

	if !reflect.DeepEqual(meta, []byte("test")) {
		t.Fatalf("bad  meta data")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	d.NodeMeta(1)
}

// internals
func TestDelegate_LocalState(t *testing.T) {
	c1 := testConfig()
	s1, err := Create(c1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	c2 := testConfig()
	s2, err := Create(c2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	yield()

	_, err = s1.Join([]string{c2.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// s2 can leave now
	s2.Leave()

	// Do a state dump
	d := c1.MemberlistConfig.Delegate
	buf := d.LocalState()

	// Verify
	if messageType(buf[0]) != messagePushPullType {
		t.Fatalf("bad message type")
	}

	// Attempt a decode
	pp := messagePushPull{}
	if err := decodeMessage(buf[1:], &pp); err != nil {
		t.Fatalf("decode failed %v", err)
	}

	// Verify lamport
	if pp.LTime != s1.clock.Time() {
		t.Fatalf("clock mismatch")
	}

	// Verify the status
	if len(pp.StatusLTimes) != 2 {
		t.Fatalf("missing ltimes")
	}

	if len(pp.LeftMembers) != 1 {
		t.Fatalf("missing left members")
	}
}
