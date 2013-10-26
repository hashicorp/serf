package command

import (
	"github.com/hashicorp/serf/cli"
	"strings"
	"testing"
)

func TestMembersCommand_implements(t *testing.T) {
	var _ cli.Command = &MembersCommand{}
}

func TestMembersCommandRun(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	c := &MembersCommand{}
	args := []string{"-rpc-addr=" + a1.RPCAddr}
	ui := new(cli.MockUi)

	code := c.Run(args, ui)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a1.SerfConfig.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommandWithJSONFlagReturnsJSON(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	c := &MembersCommand{}
	args := []string{
		"-rpc-addr=" + a1.RPCAddr,
		"-json=true",
	}
	ui := new(cli.MockUi)

	code := c.Run(args, ui)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	expectedJSON := `{"members":[{"address":"127.0.0.13","name":"127.0.0.13","role":"","status":"alive"}]}`
	actualJSON := strings.TrimSpace(ui.OutputWriter.String())
	if actualJSON != expectedJSON {
		t.Fatalf("Expected: \n\t%#v, but got: \n\t%#v", expectedJSON, actualJSON)
	}
}
