package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"io"
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"
)

// Agent actually starts and manages a Serf agent.
type Agent struct {
	EventScripts []EventScript
	LogOutput    io.Writer
	RPCAddr      string
	SerfConfig   *serf.Config

	events      []string
	eventChs    map[chan<- string]struct{}
	eventIndex  int
	eventLock   sync.Mutex
	logger      *log.Logger
	once        sync.Once
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
	a.once.Do(a.init)

	a.logger.Printf("[INFO] Agent joining: %v", addrs)
	n, err = a.serf.Join(addrs)
	return
}

// NotifyEvents causes the agent to begin sending log events to the
// given channel. The return value is a buffer of past events up to a
// point.
//
// All NotifyEvent calls should be paired with a StopEvents call.
func (a *Agent) NotifyEvents(ch chan<- string) []string {
	a.once.Do(a.init)

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
	a.once.Do(a.init)

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
	a.once.Do(a.init)

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
	a.logger.Println("[INFO] agent: requesting graceful leave from Serf")
	if err := a.serf.Leave(); err != nil {
		return err
	}

	a.logger.Println("[INFO] agent: requesting serf shutdown")
	if err := a.serf.Shutdown(); err != nil {
		return err
	}

	a.logger.Println("[INFO] agent: shutdown complete")
	a.state = AgentIdle
	close(a.shutdownCh)
	return nil
}

// Start starts the agent, kicking off any goroutines to handle various
// aspects of the agent.
func (a *Agent) Start() error {
	a.once.Do(a.init)

	a.lock.Lock()
	defer a.lock.Unlock()

	a.logger.Println("[INFO] Serf agent starting")

	for _, script := range a.EventScripts {
		if !script.Valid() {
			return fmt.Errorf("Invalid event script: %s", script)
		}
	}

	// Setup logging a bit
	a.SerfConfig.MemberlistConfig.LogOutput = a.LogOutput
	a.SerfConfig.LogOutput = a.LogOutput

	var eventCh chan serf.Event
	if len(a.EventScripts) > 0 {
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
	err = registerEndpoint(rpcServer, a)
	if err != nil {
		return fmt.Errorf("Error starting RPC server: %s", err)
	}

	go func(l net.Listener) {
		for {
			conn, err := l.Accept()
			if err != nil {
				a.logger.Printf("[ERR] RPC accept error: %s", err)
				return
			}
			go rpcServer.ServeConn(conn)
		}
	}(a.rpcListener)

	shutdownCh := make(chan struct{})

	// Only listen for events if we care about events.
	if eventCh != nil {
		go a.eventLoop(a.EventScripts, eventCh, shutdownCh)
	}

	a.shutdownCh = shutdownCh
	a.state = AgentRunning
	a.logger.Println("[INFO] Serf agent started")
	return nil
}

func (a *Agent) event(v string) {
	a.eventLock.Lock()
	defer a.eventLock.Unlock()

	if a.events == nil {
		a.events = make([]string, 512)
	}

	a.events[a.eventIndex] = v
	a.eventIndex++
	if a.eventIndex > len(a.events) {
		a.eventIndex = 0
	}

	for ch, _ := range a.eventChs {
		select {
		case ch <- v:
		default:
		}
	}
}

func (a *Agent) eventLoop(scripts []EventScript, eventCh <-chan serf.Event, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case e := <-eventCh:
			for _, script := range scripts {
				if !script.Invoke(e) {
					continue
				}

				if err := a.invokeEventScript(script.Script, e); err != nil {
					a.logger.Printf("[ERR] Error executing event script: %s", err)
				}
			}
		}
	}
}

func (a *Agent) init() {
	if a.LogOutput == nil {
		a.LogOutput = os.Stderr
	}

	eventWriter := &EventWriter{Agent: a}
	a.LogOutput = io.MultiWriter(a.LogOutput, eventWriter)

	a.logger = log.New(a.LogOutput, "", log.LstdFlags)
}
