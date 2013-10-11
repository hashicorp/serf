package agent

import (
	"encoding/gob"
	"fmt"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/serf"
	"log"
	"net"
	"net/rpc"
	"strings"
)

// registerEndpoint registers the API endpoint on the given RPC server
// for the given agent.
func registerEndpoint(s *rpc.Server, agent *Agent) error {
	return s.RegisterName("Agent", &rpcEndpoint{agent: agent})
}

// rpcEndpoint is the RPC endpoint for agent RPC calls.
type rpcEndpoint struct {
	agent *Agent
}

// RPCMonitorArgs are the args for the Monitor RPC call.
type RPCMonitorArgs struct {
	CallbackAddr string
	LogLevel     string
}

// RPCUserEventArgs are the args for the user event RPC call.
type RPCUserEventArgs struct {
	Name    string
	Payload []byte
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
func (e *rpcEndpoint) Monitor(args RPCMonitorArgs, result *interface{}) error {
	if args.LogLevel == "" {
		args.LogLevel = "DEBUG"
	}
	args.LogLevel = strings.ToUpper(args.LogLevel)

	filter := LevelFilter()
	filter.MinLevel = logutils.LogLevel(args.LogLevel)
	if !ValidateLevelFilter(filter) {
		return fmt.Errorf("Unknown log level: %s", filter.MinLevel)
	}

	go e.monitorStream(args.CallbackAddr, filter)
	return nil
}

// UserEvent requests the agent to send a user event.
func (e *rpcEndpoint) UserEvent(args RPCUserEventArgs, result *interface{}) error {
	return e.agent.UserEvent(args.Name, args.Payload)
}

func (e *rpcEndpoint) monitorStream(addr string, filter *logutils.LevelFilter) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("[ERR] Monitor connect error: %s", err)
	}
	defer conn.Close()

	eventCh := make(chan string, 128)
	defer e.agent.StopLogs(eventCh)

	enc := gob.NewEncoder(conn)
	for _, past := range e.agent.NotifyLogs(eventCh) {
		if !filter.Check([]byte(past)) {
			continue
		}

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
