package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

type UseKeyCommand struct {
	Ui cli.Ui
}

func (c *UseKeyCommand) Help() string {
	helpText := `
Usage: serf use-key [options] <key>

  Change the primary key in the keyring. The primary key is used to perform
  encryption and is the first key tried while decrypting messages.

  CAUTION! If you change the primary encryption key without first distributing
  the new key to all nodes, then nodes without the new encryption key will not
  be able to understand messages encrypted using the new key. Typically this
  means you should get an exit code of 0 from the "install-key" command before
  running "use-key".

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -rpc-auth=""              RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *UseKeyCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("use-key", flag.ContinueOnError)
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

	if err := client.UseKey(args[0]); err != nil {
		c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
		return 1
	}

	c.Ui.Output("Successfully changed primary encryption key")
	return 0
}

func (c *UseKeyCommand) Synopsis() string {
	return "Change the keyring's primary encryption key"
}
