package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/serf/cli"
	"encoding/json"
	"strings"
)

// MembersCommand is a Command implementation that queries a running
// Serf agent what members are part of the cluster currently.
type MembersCommand struct{}

func (c *MembersCommand) Help() string {
	helpText := `
Usage: serf members [options]

  Outputs the members of a running Serf agent.

Options:

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.
  -json=true                Formats the members list as a JSON object.
`
	return strings.TrimSpace(helpText)
}

func (c *MembersCommand) Run(args []string, ui cli.Ui) int {
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	JSONFormatted := jsonFormatFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	members, err := client.Members()
	if err != nil {
		ui.Error(fmt.Sprintf("Error retrieving members: %s", err))
		return 1
	}

	if *JSONFormatted != true {
		for _, member := range members {
			ui.Output(fmt.Sprintf("%s    %s    %s",
				member.Name, member.Addr, member.Status))
		}
	} else {
		jsonMembers, err := json.Marshal(map[string]interface{}{"members": members})
		if err != nil {
			ui.Error(fmt.Sprintf("Error formatting members into JSON: %s", err))
			return 1
		}

		ui.Output(string(jsonMembers))
	}

	return 0
}

func (c *MembersCommand) Synopsis() string {
	return "Lists the members of a Serf cluster"
}

// jsonFormatFlag returns a pointer to a bool that will be populated
// when the given flagset is parsed and asking for JSON format.
func jsonFormatFlag(f *flag.FlagSet) *bool {
	return f.Bool("json", false,
		"The members list formatted as a JSON object.")
}
