package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

type InstallKeyCommand struct {
	Ui cli.Ui
}

func (c *InstallKeyCommand) Help() string {
	helpText := `
Usage: serf keys [options] ...

  Manage encryption keys in use by Serf

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
  -install=<key>            Install a new key
`
	return strings.TrimSpace(helpText)
}

func (c *InstallKeyCommand) Run(args []string) int {
	var installKey string

	cmdFlags := flag.NewFlagSet("keys", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	cmdFlags.StringVar(&installKey, "install", "", "install a new key")
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	if installKey != "" {
		if err := client.InstallKey(installKey); err != nil {
			c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
			return 1
		}
		c.Ui.Output("Successfully installed new key")
		return 0
	}

	c.Ui.Error(c.Help())
	return 1
}

func (c *InstallKeyCommand) Synopsis() string {
	return "Manage encryption keys in use by Serf"
}
