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
