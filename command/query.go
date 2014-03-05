package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/command/agent"
	"github.com/mitchellh/cli"
	"strings"
	"time"
)

// QueryCommand is a Command implementation that is used to trigger a new
// query and wait for responses and acks
type QueryCommand struct {
	ShutdownCh <-chan struct{}
	Ui         cli.Ui
}

func (c *QueryCommand) Help() string {
	helpText := `
Usage: serf query [options] name payload

  Dispatches a query to the Serf cluster.

Options:

  -node=NAME                This flag can be provided multiple times to filter
                            responses to only named nodes.

  -tag key=regexp           This flag can be provided multiple times to filter
                            responses to only nodes matching the tags

  -timeout="15s"            Providing a timeout overrides the default timeout

  -no-ack                   Setting this prevents nodes from sending an acknowledgement
                            of the query

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.

  -rpc-auth=""              RPC auth token of the Serf agent.
`
	return strings.TrimSpace(helpText)
}

func (c *QueryCommand) Run(args []string) int {
	var noAck bool
	var nodes []string
	var tags []string
	var timeout time.Duration
	cmdFlags := flag.NewFlagSet("event", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.Var((*agent.AppendSliceValue)(&nodes), "node", "node filter")
	cmdFlags.Var((*agent.AppendSliceValue)(&tags), "tag", "tag filter")
	cmdFlags.DurationVar(&timeout, "timeout", 0, "query timeout")
	cmdFlags.BoolVar(&noAck, "no-ack", false, "no-ack")
	rpcAddr := RPCAddrFlag(cmdFlags)
	rpcAuth := RPCAuthFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Setup the filter tags
	filterTags := make(map[string]string)
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) != 2 {
			c.Ui.Error(fmt.Sprintf("Invalid tag '%s' provided", tag))
			return 1
		}
		filterTags[parts[0]] = parts[1]
	}

	args = cmdFlags.Args()
	if len(args) < 1 {
		c.Ui.Error("A query name must be specified.")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	} else if len(args) > 2 {
		c.Ui.Error("Too many command line arguments. Only a name and payload must be specified.")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	name := args[0]
	var payload []byte
	if len(args) == 2 {
		payload = []byte(args[1])
	}

	cl, err := RPCClient(*rpcAddr, *rpcAuth)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer cl.Close()

	ackCh := make(chan string, 128)
	respCh := make(chan client.NodeResponse, 128)

	params := client.QueryParam{
		FilterNodes: nodes,
		FilterTags:  filterTags,
		RequestAck:  !noAck,
		Timeout:     timeout,
		Name:        name,
		Payload:     payload,
		AckCh:       ackCh,
		RespCh:      respCh,
	}
	if err := cl.Query(&params); err != nil {
		c.Ui.Error(fmt.Sprintf("Error sending query: %s", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Query '%s' dispatched", name))

	// Track responses and acknowledgements
	numAcks := 0
	numResp := 0

OUTER:
	for {
		select {
		case a := <-ackCh:
			if a == "" {
				break OUTER
			}
			numAcks++
			c.Ui.Info(fmt.Sprintf("Ack from '%s'", a))

		case r := <-respCh:
			if r.From == "" {
				break OUTER
			}
			numResp++

			// Remove the trailing newline if there is one
			payload := r.Payload
			if n := len(payload); n > 0 && payload[n-1] == '\n' {
				payload = payload[:n-1]
			}

			c.Ui.Info(fmt.Sprintf("Response from '%s': %s", r.From, payload))

		case <-c.ShutdownCh:
			return 1
		}
	}

	if !noAck {
		c.Ui.Output(fmt.Sprintf("Total Acks: %d", numAcks))
	}
	c.Ui.Output(fmt.Sprintf("Total Responses: %d", numResp))
	return 0
}

func (c *QueryCommand) Synopsis() string {
	return "Send a query to the Serf cluster"
}
