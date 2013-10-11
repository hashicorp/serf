package command

import (
	"github.com/hashicorp/serf/cli"
	"strings"
	"testing"
)

func TestJoinCommand_implements(t *testing.T) {
	var _ cli.Command = &JoinCommand{}
}

func TestJoinCommandRun(t *testing.T) {
	a1 := testAgent(t)
	a2 := testAgent(t)
	defer a1.Shutdown()
	defer a2.Shutdown()

	c := &JoinCommand{}
	args := []string{
		"-rpc-addr=" + a1.RPCAddr,
		a2.SerfConfig.MemberlistConfig.BindAddr,
	}
	ui := new(cli.MockUi)

	code := c.Run(args, ui)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if len(a1.Serf().Members()) != 2 {
		t.Fatalf("bad: %#v", a1.Serf().Members())
	}
}

func TestJoinCommandRun_noAddrs(t *testing.T) {
	c := &JoinCommand{}
	args := []string{"-rpc-addr=foo"}
	ui := new(cli.MockUi)

	code := c.Run(args, ui)
	if code != 1 {
		t.Fatalf("bad: %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "one address") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
