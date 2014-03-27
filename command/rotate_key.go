package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// RotateKeyCommand is a Command implementation that tells a running Serf
// agent to join another.
type RotateKeyCommand struct {
	Ui cli.Ui
}

func (c *RotateKeyCommand) Help() string {
	helpText := `
Usage: serf rotate-key [options] key ...

  Initiates a cluster-wide encryption key rotation event to replace the
  currently used encryption key.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *RotateKeyCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("rotate-key", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	newKey := cmdFlags.Args()
	if len(newKey) != 1 {
		c.Ui.Error("Exactly one key must be specified.")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	client, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	if err := client.RotateKey(newKey[0]); err != nil {
		c.Ui.Error(fmt.Sprintf("Error rotating encryption key: %s", err))
		return 1
	}

	c.Ui.Output("Successfully rotated cluster encryption key")
	return 0
}

func (c *RotateKeyCommand) Synopsis() string {
	return "Initiate encryption key rotation for a Serf cluster"
}
