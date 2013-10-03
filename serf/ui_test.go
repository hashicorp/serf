package serf

import (
	"bytes"
	"testing"
)

func TestBasicUi_implements(t *testing.T) {
	var raw interface{}
	raw = &BasicUi{}
	if _, ok := raw.(Ui); !ok {
		t.Fatalf("should be a Ui")
	}
}

func TestBasicUi_Error(t *testing.T) {
	writer := new(bytes.Buffer)
	ui := &BasicUi{Writer: writer}
	ui.Error("HELLO")

	if writer.String() != "HELLO\n" {
		t.Fatalf("bad: %s", writer.String())
	}
}

func TestBasicUi_Output(t *testing.T) {
	writer := new(bytes.Buffer)
	ui := &BasicUi{Writer: writer}
	ui.Output("HELLO")

	if writer.String() != "HELLO\n" {
		t.Fatalf("bad: %s", writer.String())
	}
}
