package command

import (
	"encoding/base64"
	"github.com/hashicorp/serf/cli"
	"testing"
)

func TestKeygenCommand_implements(t *testing.T) {
	var _ cli.Command = &KeygenCommand{}
}

func TestKeygenCommand(t *testing.T) {
	c := &KeygenCommand{}
	ui := new(cli.MockUi)
	code := c.Run(nil, ui)
	if code != 0 {
		t.Fatalf("bad: %d", code)
	}

	output := ui.OutputWriter.String()
	result, err := base64.StdEncoding.DecodeString(output)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) != 16 {
		t.Fatalf("bad: %#v", result)
	}
}
