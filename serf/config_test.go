package serf

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.ProtocolVersion != ProtocolVersionMin {
		t.Fatalf("bad: %#v", c)
	}
}
