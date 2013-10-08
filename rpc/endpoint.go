package rpc

import (
	"github.com/hashicorp/serf/serf"
)

// endpoint is the actual net/rpc endpoint for the API.
type endpoint struct {
	serf *serf.Serf
}
