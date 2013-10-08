// The rpc package puts a net/rpc server in front of Serf. This rpc
// mechanism is used for example by the `serf` agent to communicate and
// retrieve state from the running Serf.
package rpc

import (
	"github.com/hashicorp/serf/serf"
	"io"
	"log"
	"net"
	"net/rpc"
)

type Server struct {
	listener  net.Listener
	rpcServer *rpc.Server
}

// NewServer creates a new RPC server for the given Serf instance
// and listener. It does not start the RPC server. The RPC server
// can be shut down by stopping the listener.
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

// Run runs the server, blocking forever until the listener is closed.
func (s *Server) Run() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("[ERR] Error accepting RPC connection: %s", err)
			return err
		}

		log.Printf("[DEBUG] Accepted connection: %s", conn.RemoteAddr())
		go s.ServeConn(conn)
	}
}

// ServeConn serves a single connection.
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	s.rpcServer.ServeConn(conn)
}
