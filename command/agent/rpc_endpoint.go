package agent

import (
	"encoding/gob"
	"github.com/hashicorp/serf/serf"
	"log"
	"net"
)

// rpcEndpoint is the RPC endpoint for agent RPC calls.
type rpcEndpoint struct {
	agent *Agent
}

// Join asks the Serf to join another cluster.
func (e *rpcEndpoint) Join(addrs []string, result *int) (err error) {
	*result, err = e.agent.Join(addrs)
	return
}

// Members returns the members that are currently part of the Serf.
func (e *rpcEndpoint) Members(args interface{}, result *[]serf.Member) error {
	*result = e.agent.Serf().Members()
	return nil
}

// Monitor opens a connection to the given callbackAddr and sends an event
// stream to it. This event stream is not the same as the _serf event_ stream.
// This is a general stream of events that are occuring to the agent.
func (e *rpcEndpoint) Monitor(callbackAddr string, result *interface{}) error {
	go e.monitorStream(callbackAddr)
	return nil
}

func (e *rpcEndpoint) monitorStream(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("[ERR] Monitor connect error: %s", err)
	}
	defer conn.Close()

	eventCh := make(chan string, 64)
	defer e.agent.StopEvents(eventCh)

	enc := gob.NewEncoder(conn)
	for _, past := range e.agent.NotifyEvents(eventCh) {
		if err := enc.Encode(past); err != nil {
			log.Printf("[ERR] Sending monitor event: %s", err)
			return
		}
	}

	for {
		if err := enc.Encode(<-eventCh); err != nil {
			log.Printf("[ERR] Sending monitor event: %s", err)
			return
		}
	}
}
