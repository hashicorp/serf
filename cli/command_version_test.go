package cli

import (
	"testing"
)

func TestVersionCommand_implements(t *testing.T) {
	var raw interface{}
	raw = &VersionCommand{}
	if _, ok := raw.(Command); !ok {
		t.Fatal("should be a Command")
	}
}
