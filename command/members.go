package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"strings"
)

// MembersCommand is a Command implementation that queries a running
// Serf agent what members are part of the cluster currently.
type MembersCommand struct {
	Ui cli.Ui
}

func (c *MembersCommand) Help() string {
	helpText := `
Usage: serf members [options]

  Outputs the members of a running Serf agent.

Options:

  -detailed                 Additional information such as protocol verions
                            will be shown.
  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *MembersCommand) Run(args []string) int {
	var detailed bool
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&detailed, "detailed", false, "detailed output")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	members, err := client.Members()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving members: %s", err))
		return 1
	}

	for _, member := range members {
		c.Ui.Output(fmt.Sprintf("%s    %s    %s    %s",
			member.Name, member.Addr, member.Status, member.Role))

		if detailed {
			c.Ui.Output(fmt.Sprintf("    Protocol Version: %d",
				member.DelegateCur))
			c.Ui.Output(fmt.Sprintf("    Available Protocol Range: [%d, %d]",
				member.DelegateMin, member.DelegateMax))
		}
	}

	return 0
}

func (c *MembersCommand) Synopsis() string {
	return "Lists the members of a Serf cluster"
}
