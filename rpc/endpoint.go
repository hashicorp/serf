package rpc

import (
	"github.com/hashicorp/serf/serf"
)

// endpoint is the actual net/rpc endpoint for the API.
type endpoint struct {
	serf *serf.Serf
}

// Members returns the members that are currently part of the Serf.
func (e *endpoint) Members(args interface{}, result *[]serf.Member) error {
	*result = e.serf.Members()
	return nil
}
