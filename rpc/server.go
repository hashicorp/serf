// The rpc package puts a net/rpc server in front of Serf. This rpc
// mechanism is used for example by the `serf` agent to communicate and
// retrieve state from the running Serf.
package rpc

import (
	"github.com/hashicorp/serf/serf"
	"net"
	"net/rpc"
)

type Server struct {
	listener  net.Listener
	rpcServer *rpc.Server
}

func NewServer(serf *serf.Serf, l net.Listener) (*Server, error) {
	rpcServer := rpc.NewServer()
	err := rpcServer.RegisterName("Serf", &endpoint{
		serf: serf,
	})
	if err != nil {
		return nil, err
	}

	return &Server{
		listener:  l,
		rpcServer: rpcServer,
	}, nil
}
