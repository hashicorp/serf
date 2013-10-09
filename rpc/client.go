package rpc

import (
	"github.com/hashicorp/serf/serf"
	"net/rpc"
)

type Client struct {
	rpcClient *rpc.Client
}

// NewClient returns a new Serf RPC client that uses the given
// underlying RPC client.
func NewClient(rpcClient *rpc.Client) *Client {
	return &Client{
		rpcClient: rpcClient,
	}
}

// Close just closes the underlying RPC client.
func (c *Client) Close() error {
	return c.rpcClient.Close()
}

func (c *Client) Members() ([]serf.Member, error) {
	var result []serf.Member
	err := c.rpcClient.Call("Serf.Members", new(interface{}), &result)
	return result, err
}
