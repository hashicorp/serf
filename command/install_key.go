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
Usage: serf keyring install-key [options] <newkey>

  Install a new encryption key onto Serf's internal keyring. This command will
  broadcast the new key to all nodes in the cluster. If all nodes reply that
  they have successfully received and installed the key, then the exit code will
  be 0, otherwise this command returns 1.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *InstallKeyCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("install-key", flag.ContinueOnError)
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

	if failedNodes, err := client.InstallKey(args[0]); err != nil {
		for _, node := range failedNodes {
			c.Ui.Error(fmt.Sprintf("failed: %s", node))
		}
		c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
		return 1
	}

	c.Ui.Output("Successfully installed new key")
	return 0

	c.Ui.Error(c.Help())
	return 1
}

func (c *InstallKeyCommand) Synopsis() string {
	return "Install a new encryption key onto the keyring of all members"
}
