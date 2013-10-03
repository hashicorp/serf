package cli

import (
	"testing"
)

func TestMockUi_implements(t *testing.T) {
	var raw interface{}
	raw = &MockUi{}
	if _, ok := raw.(Ui); !ok {
		t.Fatal("should be a Ui")
	}
}
