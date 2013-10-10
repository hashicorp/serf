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

	events      []string
	eventChs    map[chan<- string]struct{}
	eventIndex  int
	eventLock   sync.Mutex
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

// Join asks the Serf instance to join. See the Serf.Join function.
func (a *Agent) Join(addrs []string) (n int, err error) {
	a.event(fmt.Sprintf("Serf join request: %v", addrs))
	n, err = a.serf.Join(addrs)
	if err != nil {
		a.event(fmt.Sprintf("Serf join error: %s", err))
	} else {
		a.event(fmt.Sprintf("Serf joined %d nodes", n))
	}

	return
}

// NotifyEvents causes the agent to begin sending log events to the
// given channel. The return value is a buffer of past events up to a
// point.
//
// All NotifyEvent calls should be paired with a StopEvents call.
func (a *Agent) NotifyEvents(ch chan<- string) []string {
	a.eventLock.Lock()
	defer a.eventLock.Unlock()

	if a.eventChs == nil {
		a.eventChs = make(map[chan<- string]struct{})
	}

	a.eventChs[ch] = struct{}{}

	if a.events == nil {
		return nil
	}

	past := make([]string, 0, len(a.events))
	var endIndex int
	for i := len(a.events) - 1; i >= a.eventIndex; i-- {
		if a.events[i] != "" {
			break
		}

		endIndex = i
	}

	past = append(past, a.events[a.eventIndex:endIndex]...)
	past = append(past, a.events[:a.eventIndex]...)
	return past
}

// StopEvents causes the agent to stop sending events to the given
// channel.
func (a *Agent) StopEvents(ch chan<- string) {
	a.eventLock.Lock()
	defer a.eventLock.Unlock()

	delete(a.eventChs, ch)
}

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

	a.event("Serf agent started")

	a.shutdownCh = shutdownCh
	a.state = AgentRunning
	return nil
}

func (a *Agent) event(v string) {
	a.eventLock.Lock()
	defer a.eventLock.Unlock()

	if a.events == nil {
		a.events = make([]string, 255)
	}

	a.events[a.eventIndex] = v
	a.eventIndex++
	if a.eventIndex > len(a.events) {
		a.eventIndex = 0
	}

	for ch, _ := range a.eventChs {
		ch <- v
	}
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
