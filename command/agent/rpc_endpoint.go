package agent

import (
	"github.com/hashicorp/serf/serf"
)

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
