package rpc

import (
	"github.com/hashicorp/serf/serf"
)

// endpoint is the actual net/rpc endpoint for the API.
type endpoint struct {
	serf *serf.Serf
}

// Join asks the Serf to join another cluster.
func (e *endpoint) Join(addrs []string, result *int) (err error) {
	*result, err = e.serf.Join(addrs)
	return
}

// Members returns the members that are currently part of the Serf.
func (e *endpoint) Members(args interface{}, result *[]serf.Member) error {
	*result = e.serf.Members()
	return nil
}
