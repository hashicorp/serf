package agent

import (
	"github.com/hashicorp/serf/serf"
	"net/rpc"
)

// RPCClient is the RPC client to make requests to the agent RPC.
type RPCClient struct {
	Client *rpc.Client
}

// Close just closes the underlying RPC client.
func (c *RPCClient) Close() error {
	return c.Client.Close()
}

func (c *RPCClient) Join(addrs []string) (n int, err error) {
	err = c.Client.Call("Agent.Join", addrs, &n)
	return
}

func (c *RPCClient) Members() ([]serf.Member, error) {
	var result []serf.Member
	err := c.Client.Call("Agent.Members", new(interface{}), &result)
	return result, err
}

// rpcEndpoint is the RPC endpoint for agent RPC calls.
type rpcEndpoint struct {
	agent *Agent
}

// Join asks the Serf to join another cluster.
func (e *rpcEndpoint) Join(addrs []string, result *int) (err error) {
	*result, err = e.agent.Serf().Join(addrs)
	return
}

// Members returns the members that are currently part of the Serf.
func (e *rpcEndpoint) Members(args interface{}, result *[]serf.Member) error {
	*result = e.agent.Serf().Members()
	return nil
}
