package command

import (
	"github.com/hashicorp/serf/cli"
	"strings"
	"testing"
)

func TestEventCommand_implements(t *testing.T) {
	var _ cli.Command = &EventCommand{}
}

func TestEventCommandRun_noEvent(t *testing.T) {
	c := &EventCommand{}
	args := []string{"-rpc-addr=foo"}
	ui := new(cli.MockUi)

	code := c.Run(args, ui)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "event name") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}

func TestEventCommandRun_tooMany(t *testing.T) {
	c := &EventCommand{}
	args := []string{"-rpc-addr=foo", "foo", "bar", "baz"}
	ui := new(cli.MockUi)

	code := c.Run(args, ui)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "Too many") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
