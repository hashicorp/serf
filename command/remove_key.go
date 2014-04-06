package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

type RemoveKeyCommand struct {
	Ui cli.Ui
}

func (c *RemoveKeyCommand) Help() string {
	helpText := `
Usage: serf remove-key [options] <key>

  Remove an encryption key from Serf's internal keyring. In order for key
  removal to succeed, the key you are requesting for deletion cannot be
  the primary encryption key. You must first install a new key, use it,
  and then remove the old key.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *RemoveKeyCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("remove-key", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	args = cmdFlags.Args()
	if len(args) != 1 {
		c.Ui.Error("Exactly one key must be provided")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	if args[0] == "" {
		c.Ui.Error(c.Help())
		return 1
	}

	client, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	if failedNodes, err := client.RemoveKey(args[0]); err != nil {
		for _, node := range failedNodes {
			c.Ui.Error(fmt.Sprintf("failed: %s", node))
		}
		c.Ui.Error(fmt.Sprintf("Error removing key: %s", err))
		return 1
	}

	c.Ui.Output("Successfully removed encryption key")
	return 0
}

func (c *RemoveKeyCommand) Synopsis() string {
	return "Remove an encryption key from Serf's internal keyring."
}
