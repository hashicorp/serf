package serf

import (
	"github.com/hashicorp/memberlist"
	"reflect"
	"testing"
)

func TestMemberlistConfig(t *testing.T) {
	d := DefaultConfig()
	mlC := memberlistConfig(d)
	mlDefault := memberlist.DefaultConfig()
	if !reflect.DeepEqual(mlDefault, mlC) {
		t.Fatalf("default config does not match")
	}
}
