package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

type KeyCommand struct {
	Ui cli.Ui
}

func (c *KeyCommand) Help() string {
	helpText := `
Usage: serf keys [options]...

  Manipulates the internal encryption keyring used by Serf. This command
  supports chaining arguments to allow performing a complete key rotation
  in a single command.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
  -install=<key>            Install a new key onto Serf's internal keyring.
  -use=<key>                Change the primary key used for encrypting messages.
  -remove=<key>             Remove a key from Serf's internal keyring.
`
	return strings.TrimSpace(helpText)
}

func (c *KeyCommand) Run(args []string) int {
	var installKey, useKey, removeKey string

	cmdFlags := flag.NewFlagSet("install-key", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&installKey, "install", "", "install a new key")
	cmdFlags.StringVar(&useKey, "use", "", "change primary encryption key")
	cmdFlags.StringVar(&removeKey, "remove", "", "remove a key")
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	if fmt.Sprintf("%s%s%s", installKey, useKey, removeKey) == "" {
		c.Ui.Error("At least one of -install, -use, or -remove required")
		return 1
	}

	if installKey != "" {
		if failedNodes, err := client.InstallKey(installKey); err != nil {
			for _, node := range failedNodes {
				c.Ui.Error(fmt.Sprintf("failed: %s", node))
			}
			c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
			return 1
		}
		c.Ui.Output("Successfully installed key")
	}

	if useKey != "" {
		if failedNodes, err := client.UseKey(useKey); err != nil {
			for _, node := range failedNodes {
				c.Ui.Error(fmt.Sprintf("failed: %s", node))
			}
			c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
			return 1
		}
		c.Ui.Output("Successfully changed primary key")
	}

	if removeKey != "" {
		if failedNodes, err := client.RemoveKey(removeKey); err != nil {
			for _, node := range failedNodes {
				c.Ui.Error(fmt.Sprintf("failed: %s", node))
			}
			c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
			return 1
		}
		c.Ui.Output("Successfully removed key")
	}

	return 0
}

func (c *KeyCommand) Synopsis() string {
	return "Manipulate the internal encryption keyring used by Serf"
}
