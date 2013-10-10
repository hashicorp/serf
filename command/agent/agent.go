package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"log"
	"net"
	"net/rpc"
	"sync"
)

// Agent actually starts and manages a Serf agent.
type Agent struct {
	EventScript string
	RPCAddr     string
	SerfConfig  *serf.Config

	rpcListener net.Listener
	serf        *serf.Serf
	shutdownCh  chan<- struct{}
	state       AgentState
	lock        sync.Mutex
}

type AgentState int

const (
	AgentIdle AgentState = iota
	AgentRunning
)

// Returns the Serf agent of the running Agent.
func (a *Agent) Serf() *serf.Serf {
	return a.serf
}

// Shutdown does a graceful shutdown of this agent and all of its processes.
func (a *Agent) Shutdown() error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.state == AgentIdle {
		return nil
	}

	// Stop the RPC listener which in turn will stop the RPC server.
	if err := a.rpcListener.Close(); err != nil {
		return err
	}

	// Gracefully leave the serf cluster
	log.Println("[INFO] agent: requesting graceful leave from Serf")
	if err := a.serf.Leave(); err != nil {
		return err
	}

	log.Println("[INFO] agent: requesting serf shutdown")
	if err := a.serf.Shutdown(); err != nil {
		return err
	}

	log.Println("[INFO] agent: shutdown complete")
	a.state = AgentIdle
	close(a.shutdownCh)
	return nil
}

// Start starts the agent, kicking off any goroutines to handle various
// aspects of the agent.
func (a *Agent) Start() error {
	a.lock.Lock()
	defer a.lock.Unlock()

	var eventCh chan serf.Event
	if a.EventScript != "" {
		eventCh = make(chan serf.Event, 64)
		a.SerfConfig.EventCh = eventCh
	}

	var err error
	a.serf, err = serf.Create(a.SerfConfig)
	if err != nil {
		return fmt.Errorf("Error creating Serf: %s", err)
	}

	a.rpcListener, err = net.Listen("tcp", a.RPCAddr)
	if err != nil {
		return fmt.Errorf("Error starting RPC listener: %s", err)
	}

	rpcServer := rpc.NewServer()
	err = rpcServer.RegisterName("Agent", &rpcEndpoint{agent: a})
	if err != nil {
		return fmt.Errorf("Error starting RPC server: %s", err)
	}

	go func(l net.Listener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("[ERR] RPC accept error: %s", err)
				return
			}
			go rpcServer.ServeConn(conn)
		}
	}(a.rpcListener)

	shutdownCh := make(chan struct{})

	// Only listen for events if we care about events.
	if eventCh != nil {
		go a.eventLoop(a.EventScript, eventCh, shutdownCh)
	}

	a.shutdownCh = shutdownCh
	a.state = AgentRunning
	return nil
}

func (a *Agent) eventLoop(script string, eventCh <-chan serf.Event, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case e := <-eventCh:
			if err := invokeEventScript(script, &e); err != nil {
				log.Printf("[ERR] Error executing event script: %s", err)
			}
		}
	}
}
