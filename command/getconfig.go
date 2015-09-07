package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// GetConfigCommand is a Command implementation that queries a running
// Serf agent for the current config
type GetConfigCommand struct {
	Ui cli.Ui
}

func (i *GetConfigCommand) Help() string {
	helpText := `
Usage: serf getconfig [options]

	Dumps the active config data for the running serf agent

Options:

  -rpc-addr=127.0.0.1:7373 RPC address of the Serf agent.

  -rpc-auth=""             RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (i *GetConfigCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("config", flag.ContinueOnError)
	cmdFlags.Usage = func() { i.Ui.Output(i.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		i.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	config_data, err := client.GetConfig()
	if err != nil {
		i.Ui.Error(fmt.Sprintf("Error querying agent: %s", err))
		return 1
	}

	i.Ui.Output(config_data)
	return 0
}

func (i *GetConfigCommand) Synopsis() string {
	return "Dumps the config for the running serf agent"
}
