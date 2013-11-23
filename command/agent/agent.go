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
// invoking and EventHandler when events occur, and putting an RPC
// layer in front so that you can query and control the Serf instance
// remotely.
type Agent struct {
	// eventCh is used for Serf to deliver events on
	eventCh chan serf.Event

	// eventHandlers is the registered handlers for events
	eventHandlers     map[EventHandler]struct{}
	eventHandlersLock sync.Mutex

	// logWriter is used to buffer and handle log streaming
	logWriter *logWriter

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
func Start(conf *serf.Config, logOutput io.Writer) (*Agent, error) {
	// Ensure we have a log sink
	if logOutput == nil {
		logOutput = os.Stderr
	}

	// Wrap the log output to buffer logs
	logWriter := newLogWriter(512)
	logOutput = io.MultiWriter(logOutput, logWriter)

	// Setup the underlying loggers
	conf.MemberlistConfig.LogOutput = logOutput
	conf.LogOutput = logOutput

	// Create a channel to listen for events from Serf
	eventCh := make(chan serf.Event, 64)
	conf.EventCh = eventCh

	// Create serf first
	serf, err := serf.Create(conf)
	if err != nil {
		return nil, fmt.Errorf("Error creating Serf: %s", err)
	}

	// Setup the agent
	agent := &Agent{
		eventCh:       eventCh,
		eventHandlers: make(map[EventHandler]struct{}),
		logWriter:     logWriter,
		logger:        log.New(logOutput, "", log.LstdFlags),
		serf:          serf,
		shutdownCh:    make(chan struct{}),
	}
	go agent.eventLoop()
	agent.logger.Printf("[INFO] Serf agent started")
	return agent, nil
}

// Shutdown does a graceful shutdown of this agent and all of its processes.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
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
	a.shutdown = true
	close(a.shutdownCh)
	return nil
}

// Returns the Serf agent of the running Agent.
func (a *Agent) Serf() *serf.Serf {
	return a.serf
}

// Join asks the Serf instance to join. See the Serf.Join function.
func (a *Agent) Join(addrs []string, replay bool) (n int, err error) {
	a.logger.Printf("[INFO] Agent joining: %v replay: %v", addrs, replay)
	ignoreOld := !replay
	return a.serf.Join(addrs, ignoreOld)
}

// ForceLeave is used to eject a failed node from the cluster
func (a *Agent) ForceLeave(node string) error {
	a.logger.Printf("[INFO] Force leaving node: %s", node)
	return a.serf.RemoveFailedNode(node)
}

// UserEvent sends a UserEvent on Serf, see Serf.UserEvent.
func (a *Agent) UserEvent(name string, payload []byte, coalesce bool) error {
	a.logger.Printf("[DEBUG] Requesting user event send: %s. Coalesced: %#v. Payload: %#v",
		name, coalesce, string(payload))
	return a.serf.UserEvent(name, payload, coalesce)
}

// RegisterLogHandler adds a log handler to recieve logs, and sends
// the last buffered logs to the handler
func (a *Agent) RegisterLogHandler(lh LogHandler) {
	a.logWriter.RegisterHandler(lh)
}

// DeregisterLogHandler removes a LogHandler and prevents more invocations
func (a *Agent) DeregisterLogHandler(lh LogHandler) {
	a.logWriter.DeregisterHandler(lh)
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
