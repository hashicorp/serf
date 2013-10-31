package agent

import (
	"encoding/gob"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/serf"
	"log"
	"net"
	"net/rpc"
	"sync"
)

// RPCClient is the RPC client to make requests to the agent RPC.
type RPCClient struct {
	Client *rpc.Client
}

// Close just closes the underlying RPC client.
func (c *RPCClient) Close() error {
	return c.Client.Close()
}

func (c *RPCClient) ForceLeave(node string) error {
	return c.Client.Call("Agent.ForceLeave", node, new(interface{}))
}

func (c *RPCClient) Join(addrs []string, ignoreOld bool) (n int, err error) {
	args := RPCJoinArgs{
		Addrs:     addrs,
		IgnoreOld: ignoreOld,
	}

	err = c.Client.Call("Agent.Join", args, &n)
	return
}

func (c *RPCClient) Members() ([]serf.Member, error) {
	var result []serf.Member
	err := c.Client.Call("Agent.Members", new(interface{}), &result)
	return result, err
}

func (c *RPCClient) UserEvent(name string, payload []byte, coalesce bool) error {
	return c.Client.Call("Agent.UserEvent", RPCUserEventArgs{
		Name:     name,
		Payload:  payload,
		Coalesce: coalesce,
	}, new(interface{}))
}

func (c *RPCClient) Monitor(level logutils.LogLevel, ch chan<- string, done <-chan struct{}) error {
	var conn net.Conn
	var connClosed bool
	var l net.Listener
	var lClosed bool
	var lock sync.Mutex
	internalDone := make(chan struct{})

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	// Make the RPC call that will call back to us
	args := RPCMonitorArgs{
		CallbackAddr: l.Addr().String(),
		LogLevel:     string(level),
	}
	err = c.Client.Call("Agent.Monitor", args, new(interface{}))
	if err != nil {
		return err
	}

	go func() {
		select {
		case <-done:
		case <-internalDone:
		}

		lock.Lock()
		defer lock.Unlock()

		if l != nil && !lClosed {
			l.Close()
			lClosed = true
		}

		if conn != nil && !connClosed {
			conn.Close()
			connClosed = true
		}
	}()

	go func() {
		defer close(internalDone)

		// Accept the connection that the RPC server will make to us
		var err error
		conn, err = l.Accept()

		// Close the listener right away, we only accept one connection
		lock.Lock()
		if !lClosed {
			l.Close()
			lClosed = true
		}
		lock.Unlock()

		if err != nil {
			log.Printf("[ERR] Failed to accept monitor connection: %s", err)
			return
		}

		var event string
		dec := gob.NewDecoder(conn)
		for {
			if err := dec.Decode(&event); err != nil {
				log.Printf("[ERR] Error decoding monitor event: %s", err)
				close(ch)
				return
			}

			ch <- event
		}
	}()

	return nil
}
