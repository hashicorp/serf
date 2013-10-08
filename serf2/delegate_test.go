package serf2

import (
	"github.com/hashicorp/memberlist"
	"testing"
)

func TestDelegate_impl(t *testing.T) {
	var raw interface{}
	raw = new(delegate)
	if _, ok := raw.(memberlist.Delegate); !ok {
		t.Fatal("should be an Delegate")
	}
}
