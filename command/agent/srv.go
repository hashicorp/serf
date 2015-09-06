package agent

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const (
	srvPollInterval  = 60 * time.Second
	srvQuietInterval = 100 * time.Millisecond
)

// AgentSRV periodically polls an SRV record
// And attempts to join the hosts supplied
type AgentSRV struct {
	agent   *Agent
	srvrecords string
	logger  *log.Logger
	replay  bool
}

// NewAgentSRV is used to create a new AgentSRV
func NewAgentSRV(agent *Agent, logOutput io.Writer, replay bool, srvrecords string) (*AgentSRV, error) {

	// Initialize the AgentSRV
	m := &AgentSRV{
		agent:   agent,
		srvrecords: srvrecords,
		logger:  log.New(logOutput, "", log.LstdFlags),
		replay:  replay,
	}

	// Start the background workers
	go m.run()
	return m, nil
}

// run is a long running goroutine that scans for new hosts periodically
func (m *AgentSRV) run() {
	hosts := make(chan *net.SRV)
	poll := time.After(0)
	var quiet <-chan time.Time
	var join []string

	for {
		select {
		case h := <-hosts:
			// Format the host address
			addr := fmt.Sprintf("%s:%d", h.Target, h.Port)

			// Queue for handling
			join = append(join, addr)
			quiet = time.After(srvQuietInterval)

		case <-quiet:
			// Attempt the join
			n, err := m.agent.Join(join, m.replay)
			if err != nil {
				m.logger.Printf("[ERR] agent.srv: Failed to join: %v", err)
			}
			if n > 0 {
				m.logger.Printf("[INFO] agent.srv: Joined %d hosts", n)
			}

			join = nil

		case <-poll:
			poll = time.After(srvPollInterval)
			go m.poll(hosts)
		}
	}
}

// poll is invoked periodically to check for new hosts
func (m *AgentSRV) poll(hosts chan *net.SRV) {
	for _, record := range strings.Split(m.srvrecords, ",") {
		_, results, err := net.LookupSRV("", "", record)

		if err != nil {
			m.logger.Printf("[ERR] agent.srv: Failed to poll for new hosts: %v", err)
		}

		for _, host := range results {
			hosts <- host
		}
	}
}
