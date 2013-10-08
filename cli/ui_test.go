package cli

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

func TestPrefixedUi_implements(t *testing.T) {
	var raw interface{}
	raw = &PrefixedUi{}
	if _, ok := raw.(Ui); !ok {
		t.Fatalf("should be a Ui")
	}
}

func TestPrefixedUiError(t *testing.T) {
	ui := new(MockUi)
	p := &PrefixedUi{
		ErrorPrefix: "foo",
		Ui:          ui,
	}

	p.Error("bar")
	if ui.ErrorWriter.String() != "foobar\n" {
		t.Fatalf("bad: %s", ui.ErrorWriter.String())
	}
}

func TestPrefixedUiInfo(t *testing.T) {
	ui := new(MockUi)
	p := &PrefixedUi{
		InfoPrefix: "foo",
		Ui:         ui,
	}

	p.Info("bar")
	if ui.OutputWriter.String() != "foobar\n" {
		t.Fatalf("bad: %s", ui.OutputWriter.String())
	}
}

func TestPrefixedUiOutput(t *testing.T) {
	ui := new(MockUi)
	p := &PrefixedUi{
		OutputPrefix: "foo",
		Ui:           ui,
	}

	p.Output("bar")
	if ui.OutputWriter.String() != "foobar\n" {
		t.Fatalf("bad: %s", ui.OutputWriter.String())
	}
}
