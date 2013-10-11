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

// Agent starts and manages a Serf instance, adding some niceties
// on top of Serf such as storing logs that you can later retrieve,
// invoking and EventHandler when events occur, and putting an RPC
// layer in front so that you can query and control the Serf instance
// remotely.
type Agent struct {
	// EventHandler is what is invoked when an event occurs. If this
	// isn't set, then events aren't handled in any special way.
	EventHandler EventHandler

	// LogOutput is where log messages are written to for the Agent,
	// Serf, and the underlying Memberlist.
	LogOutput io.Writer

	// RPCAddr is the address to bind the RPC listener to.
	RPCAddr string

	// SerfConfig is the configuration of Serf to use. Some settings
	// in here may be overriden by the agent.
	SerfConfig *serf.Config

	logs        []string
	logChs      map[chan<- string]struct{}
	logIndex    int
	logLock     sync.Mutex
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

// NotifyLogs causes the agent to begin sending log events to the
// given channel. The return value is a buffer of past events up to a
// point.
//
// All NotifyLogs calls should be paired with a StopLogs call.
func (a *Agent) NotifyLogs(ch chan<- string) []string {
	a.once.Do(a.init)

	a.logLock.Lock()
	defer a.logLock.Unlock()

	if a.logChs == nil {
		a.logChs = make(map[chan<- string]struct{})
	}

	a.logChs[ch] = struct{}{}

	if a.logs == nil {
		return nil
	}

	past := make([]string, 0, len(a.logs))
	if a.logs[a.logIndex] != "" {
		past = append(past, a.logs[a.logIndex:]...)
	}
	past = append(past, a.logs[:a.logIndex]...)
	return past
}

// StopLogs causes the agent to stop sending logs to the given
// channel.
func (a *Agent) StopLogs(ch chan<- string) {
	a.once.Do(a.init)

	a.logLock.Lock()
	defer a.logLock.Unlock()

	delete(a.logChs, ch)
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

	// Setup logging a bit
	a.SerfConfig.MemberlistConfig.LogOutput = a.LogOutput
	a.SerfConfig.LogOutput = a.LogOutput

	eventCh := make(chan serf.Event, 64)
	a.SerfConfig.EventCh = eventCh

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
	go a.eventLoop(a.EventHandler, eventCh, shutdownCh)

	a.shutdownCh = shutdownCh
	a.state = AgentRunning
	a.logger.Println("[INFO] Serf agent started")
	return nil
}

// UserEvent sends a UserEvent on Serf, see Serf.UserEvent.
func (a *Agent) UserEvent(name string, payload []byte) error {
	a.logger.Printf("Requesting user event send: %s %#v",
		name, string(payload))
	return a.serf.UserEvent(name, payload)
}

func (a *Agent) storeLog(v string) {
	a.logLock.Lock()
	defer a.logLock.Unlock()

	if a.logs == nil {
		a.logs = make([]string, 512)
	}

	a.logs[a.logIndex] = v
	a.logIndex++
	if a.logIndex >= len(a.logs) {
		a.logIndex = 0
	}

	for ch, _ := range a.logChs {
		select {
		case ch <- v:
		default:
		}
	}
}

func (a *Agent) eventLoop(h EventHandler, eventCh <-chan serf.Event, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case e := <-eventCh:
			a.logger.Printf("[INFO] agent: Received event: %s", e.String())

			if h == nil {
				continue
			}

			err := h.HandleEvent(a.logger, e)
			if err != nil {
				a.logger.Printf("[ERR] agent: Error invoking event handler: %s", err)
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
