package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/serf/cli"
	"strings"
)

// EventCommand is a Command implementation that queries a running
// Serf agent what members are part of the cluster currently.
type EventCommand struct{}

func (c *EventCommand) Help() string {
	helpText := `
Usage: serf event [options] name payload

  Dispatches a custom event across the Serf cluster.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *EventCommand) Run(args []string, ui cli.Ui) int {
	cmdFlags := flag.NewFlagSet("event", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	args = cmdFlags.Args()
	if len(args) < 1 {
		ui.Error("An event name must be specified.")
		ui.Error("")
		ui.Error(c.Help())
		return 1
	} else if len(args) > 2 {
		ui.Error("Too many command line arguments. Only a name and payload must be specified.")
		ui.Error("")
		ui.Error(c.Help())
		return 1
	}

	event := args[0]
	var payload []byte
	if len(args) == 2 {
		payload = []byte(args[1])
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	if err := client.UserEvent(event, payload); err != nil {
		ui.Error(fmt.Sprintf("Error sending event: %s", err))
		return 1
	}

	return 0
}

func (c *EventCommand) Synopsis() string {
	return "Send a custom event through the Serf cluster"
}
