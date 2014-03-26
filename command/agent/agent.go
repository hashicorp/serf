package agent

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/serf/serf"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

// Agent starts and manages a Serf instance, adding some niceties
// on top of Serf such as storing logs that you can later retrieve,
// and invoking EventHandlers when events occur.
type Agent struct {
	// Stores the serf configuration
	conf *serf.Config

	// Stores the agent configuration
	agentConf *Config

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
func Create(agentConf *Config, conf *serf.Config, logOutput io.Writer) (*Agent, error) {
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
		agentConf:     agentConf,
		eventCh:       eventCh,
		eventHandlers: make(map[EventHandler]struct{}),
		logger:        log.New(logOutput, "", log.LstdFlags),
		shutdownCh:    make(chan struct{}),
	}

	// Restore agent tags from a tags file
	if agentConf.TagsFile != "" {
		if err := agent.loadTagsFile(agentConf.TagsFile); err != nil {
			return nil, err
		}
	}

	return agent, nil
}

// Start is used to initiate the event listeners. It is seperate from
// create so that there isn't a race condition between creating the
// agent and registering handlers
func (a *Agent) Start() error {
	a.logger.Printf("[INFO] agent: Serf agent starting")

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
	if n > 0 {
		a.logger.Printf("[INFO] agent: joined: %d nodes", n)
	}
	if err != nil {
		a.logger.Printf("[WARN] agent: error joining: %v", err)
	}
	return
}

// ForceLeave is used to eject a failed node from the cluster
func (a *Agent) ForceLeave(node string) error {
	a.logger.Printf("[INFO] agent: Force leaving node: %s", node)
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
	err := a.serf.UserEvent(name, payload, coalesce)
	if err != nil {
		a.logger.Printf("[WARN] agent: failed to send user event: %v", err)
	}
	return err
}

// Query sends a Query on Serf, see Serf.Query.
func (a *Agent) Query(name string, payload []byte, params *serf.QueryParam) (*serf.QueryResponse, error) {
	// Prevent the use of the internal prefix
	if strings.HasPrefix(name, serf.InternalQueryPrefix) {
		// Allow the special "ping" query
		if name != serf.InternalQueryPrefix+"ping" || payload != nil {
			return nil, fmt.Errorf("Queries cannot contain the '%s' prefix", serf.InternalQueryPrefix)
		}
	}
	a.logger.Printf("[DEBUG] agent: Requesting query send: %s. Payload: %#v",
		name, string(payload))
	resp, err := a.serf.Query(name, payload, params)
	if err != nil {
		a.logger.Printf("[WARN] agent: failed to start user query: %v", err)
	}
	return resp, err
}

// RotateKey initiates the process of rotating the encryption key
func (a *Agent) RotateKey(newKey string) (n int, err error) {
	totalMembers := len(a.serf.Members())

	a.logger.Printf("[INFO] agent: initiating key rotation on %d nodes", totalMembers)

	n, err = a.serf.RotateKey(newKey)
	if err != nil {
		a.logger.Printf("[WARN] agent: error rotating key: %v", err)
		return
	}

	return
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
	serfShutdownCh := a.serf.ShutdownCh()
	for {
		select {
		case e := <-a.eventCh:
			a.logger.Printf("[INFO] agent: Received event: %s", e.String())
			a.eventHandlersLock.Lock()
			for eh, _ := range a.eventHandlers {
				eh.HandleEvent(e)
			}
			a.eventHandlersLock.Unlock()

		case <-serfShutdownCh:
			a.logger.Printf("[WARN] agent: Serf shutdown detected, quitting")
			a.Shutdown()
			return

		case <-a.shutdownCh:
			return
		}
	}
}

// SetTags is used to update the tags. The agent will make sure to
// persist tags if necessary before gossiping to the cluster.
func (a *Agent) SetTags(tags map[string]string) error {
	// Update the tags file if we have one
	if a.agentConf.TagsFile != "" {
		if err := a.writeTagsFile(tags); err != nil {
			a.logger.Printf("[ERR] agent: %s", err)
			return err
		}
	}

	// Set the tags in Serf, start gossiping out
	return a.serf.SetTags(tags)
}

// loadTagsFile will load agent tags out of a file and set them in the
// current serf configuration.
func (a *Agent) loadTagsFile(tagsFile string) error {
	// Avoid passing tags and using a tags file at the same time
	if len(a.agentConf.Tags) > 0 {
		return fmt.Errorf("Tags config not allowed while using tag files")
	}

	if _, err := os.Stat(tagsFile); err == nil {
		tagData, err := ioutil.ReadFile(tagsFile)
		if err != nil {
			return fmt.Errorf("Failed to read tags file: %s", err)
		}
		if err := json.Unmarshal(tagData, &a.conf.Tags); err != nil {
			return fmt.Errorf("Failed to decode tags file: %s", err)
		}
		a.logger.Printf("[INFO] agent: Restored %d tag(s) from %s",
			len(a.conf.Tags), tagsFile)
	}

	// Success!
	return nil
}

// writeTagsFile will write the current tags to the configured tags file.
func (a *Agent) writeTagsFile(tags map[string]string) error {
	encoded, err := json.MarshalIndent(tags, "", "  ")
	if err != nil {
		return fmt.Errorf("Failed to encode tags: %s", err)
	}

	// Use 0600 for permissions, in case tag data is sensitive
	if err = ioutil.WriteFile(a.agentConf.TagsFile, encoded, 0600); err != nil {
		return fmt.Errorf("Failed to write tags file: %s", err)
	}

	// Success!
	return nil
}

// MarshalTags is a utility function which takes a map of tag key/value pairs
// and returns the same tags as strings in 'key=value' format.
func MarshalTags(tags map[string]string) []string {
	var result []string
	for name, value := range tags {
		result = append(result, fmt.Sprintf("%s=%s", name, value))
	}
	return result
}

// UnmarshalTags is a utility function which takes a slice of strings in
// key=value format and returns them as a tag mapping.
func UnmarshalTags(tags []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid tag: '%s'", tag)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}
