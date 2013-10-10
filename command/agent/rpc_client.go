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
