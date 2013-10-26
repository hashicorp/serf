package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/serf/cli"
	"strings"
)

// ForceLeaveCommand is a Command implementation that tells a running Serf
// to force a member to enter the "left" state.
type ForceLeaveCommand struct{}

func (c *ForceLeaveCommand) Run(args []string, ui cli.Ui) int {
	cmdFlags := flag.NewFlagSet("join", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	nodes := cmdFlags.Args()
	if len(nodes) != 1 {
		ui.Error("A node name must be specified to force leave.")
		ui.Error("")
		ui.Error(c.Help())
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	err = client.ForceLeave(nodes[0])
	if err != nil {
		ui.Error(fmt.Sprintf("Error force leaving: %s", err))
		return 1
	}

	return 0
}

func (c *ForceLeaveCommand) Synopsis() string {
	return "Forces a member of the cluster to enter the \"left\" state"
}

func (c *ForceLeaveCommand) Help() string {
	helpText := `
Usage: serf force-leave [options] name

  Forces a member of a Serf cluster to enter the "left" state. Note
  that if the member is still actually alive, it will eventually rejoin
  the cluster. This command is most useful for cleaning out "failed" nodes
  that are never coming back. If you do not force leave a failed node,
  Serf will attempt to reconnect to those failed nodes for some period of
  time before eventually reaping them.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.

`
	return strings.TrimSpace(helpText)
}
