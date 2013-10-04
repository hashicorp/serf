package cli

import (
	"testing"
)

func TestMockCommand_implements(t *testing.T) {
	var raw interface{}
	raw = &MockCommand{}
	if _, ok := raw.(Command); !ok {
		t.Fatal("should be a Command")
	}
}
