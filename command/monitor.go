package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/cli"
	"strings"
)

// MonitorCommand is a Command implementation that queries a running
// Serf agent what members are part of the cluster currently.
type MonitorCommand struct {
	ShutdownCh <-chan struct{}
}

func (c *MonitorCommand) Help() string {
	helpText := `
Usage: serf monitor [options]

  Shows recent events of a Serf agent, and monitors the agent, outputting
  events as they occur in real time.

Options:

  -log-level=info          Log level of the agent.
  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *MonitorCommand) Run(args []string, ui cli.Ui) int {
	var logLevel string
	cmdFlags := flag.NewFlagSet("monitor", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.StringVar(&logLevel, "log-level", "INFO", "log level")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	eventCh := make(chan string)
	doneCh := make(chan struct{})
	if err := client.Monitor(logutils.LogLevel(logLevel), eventCh, doneCh); err != nil {
		ui.Error(fmt.Sprintf("Error starting monitor: %s", err))
		return 1
	}

	for {
		select {
		case e := <-eventCh:
			ui.Info(e)
		case <-c.ShutdownCh:
			close(doneCh)
			return 0
		}
	}

	return 0
}

func (c *MonitorCommand) Synopsis() string {
	return "Stream events from a Serf agent"
}
