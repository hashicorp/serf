package cli

import (
	"testing"
)

func TestMembersCommand_implements(t *testing.T) {
	var raw interface{}
	raw = &MembersCommand{}
	if _, ok := raw.(Command); !ok {
		t.Fatal("should be a Command")
	}
}
