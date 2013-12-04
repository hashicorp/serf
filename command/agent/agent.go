package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"io"
	"log"
	"os"
	"sync"
)

// Agent starts and manages a Serf instance, adding some niceties
// on top of Serf such as storing logs that you can later retrieve,
// and invoking EventHandlers when events occur.
type Agent struct {
	// Stores the serf configuration
	conf *serf.Config

	// eventCh is used for Serf to deliver events on
	eventCh chan serf.Event

	// eventHandlers is the registered handlers for events
	eventHandlers     map[EventHandler]struct{}
	eventHandlersLock sync.Mutex

	// logger instance wraps the logOutput
	logger *log.Logger

	// This is the underlying Serf we are wrapping
	serf *serf.Serf

	// shutdownCh is used for shutdowns
	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// Start creates a new agent, potentially returning an error
func Create(conf *serf.Config, logOutput io.Writer) (*Agent, error) {
	// Ensure we have a log sink
	if logOutput == nil {
		logOutput = os.Stderr
	}

	// Setup the underlying loggers
	conf.MemberlistConfig.LogOutput = logOutput
	conf.LogOutput = logOutput

	// Create a channel to listen for events from Serf
	eventCh := make(chan serf.Event, 64)
	conf.EventCh = eventCh

	// Setup the agent
	agent := &Agent{
		conf:          conf,
		eventCh:       eventCh,
		eventHandlers: make(map[EventHandler]struct{}),
		logger:        log.New(logOutput, "", log.LstdFlags),
		shutdownCh:    make(chan struct{}),
	}
	return agent, nil
}

// Start is used to initiate the event listeners. It is seperate from
// create so that there isn't a race condition between creating the
// agent and registering handlers
func (a *Agent) Start() error {
	a.logger.Printf("[INFO] Serf agent starting")

	// Create serf first
	serf, err := serf.Create(a.conf)
	if err != nil {
		return fmt.Errorf("Error creating Serf: %s", err)
	}
	a.serf = serf

	// Start event loop
	go a.eventLoop()
	return nil
}

// Leave prepares for a graceful shutdown of the agent and its processes
func (a *Agent) Leave() error {
	if a.serf == nil {
		return nil
	}

	a.logger.Println("[INFO] agent: requesting graceful leave from Serf")
	return a.serf.Leave()
}

// Shutdown closes this agent and all of its processes. Should be preceeded
// by a Leave for a graceful shutdown.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}

	if a.serf == nil {
		goto EXIT
	}

	a.logger.Println("[INFO] agent: requesting serf shutdown")
	if err := a.serf.Shutdown(); err != nil {
		return err
	}

EXIT:
	a.logger.Println("[INFO] agent: shutdown complete")
	a.shutdown = true
	close(a.shutdownCh)
	return nil
}

// ShutdownCh returns a channel that can be selected to wait
// for the agent to perform a shutdown.
func (a *Agent) ShutdownCh() <-chan struct{} {
	return a.shutdownCh
}

// Returns the Serf agent of the running Agent.
func (a *Agent) Serf() *serf.Serf {
	return a.serf
}

// Returns the Serf config of the running Agent.
func (a *Agent) SerfConfig() *serf.Config {
	return a.conf
}

// Join asks the Serf instance to join. See the Serf.Join function.
func (a *Agent) Join(addrs []string, replay bool) (n int, err error) {
	a.logger.Printf("[INFO] agent: joining: %v replay: %v", addrs, replay)
	ignoreOld := !replay
	n, err = a.serf.Join(addrs, ignoreOld)
	a.logger.Printf("[INFO] agent: joined: %d Err: %v", n, err)
	return
}

// ForceLeave is used to eject a failed node from the cluster
func (a *Agent) ForceLeave(node string) error {
	a.logger.Printf("[INFO] Force leaving node: %s", node)
	err := a.serf.RemoveFailedNode(node)
	if err != nil {
		a.logger.Printf("[WARN] agent: failed to remove node: %v", err)
	}
	return err
}

// UserEvent sends a UserEvent on Serf, see Serf.UserEvent.
func (a *Agent) UserEvent(name string, payload []byte, coalesce bool) error {
	a.logger.Printf("[DEBUG] agent: Requesting user event send: %s. Coalesced: %#v. Payload: %#v",
		name, coalesce, string(payload))
	return a.serf.UserEvent(name, payload, coalesce)
}

// RegisterEventHandler adds an event handler to recieve event notifications
func (a *Agent) RegisterEventHandler(eh EventHandler) {
	a.eventHandlersLock.Lock()
	defer a.eventHandlersLock.Unlock()
	a.eventHandlers[eh] = struct{}{}
}

// DeregisterEventHandler removes an EventHandler and prevents more invocations
func (a *Agent) DeregisterEventHandler(eh EventHandler) {
	a.eventHandlersLock.Lock()
	defer a.eventHandlersLock.Unlock()
	delete(a.eventHandlers, eh)
}

// eventLoop listens to events from Serf and fans out to event handlers
func (a *Agent) eventLoop() {
	for {
		select {
		case e := <-a.eventCh:
			a.logger.Printf("[INFO] agent: Received event: %s", e.String())
			a.eventHandlersLock.Lock()
			for eh, _ := range a.eventHandlers {
				eh.HandleEvent(e)
			}
			a.eventHandlersLock.Unlock()

		case <-a.shutdownCh:
			return
		}
	}
}
