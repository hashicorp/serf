package agent

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const (
	srvPollInterval = 60 * time.Second
)

// AgentSRV periodically polls an SRV record
// And attempts to join the hosts supplied
type AgentSRV struct {
	agent      *Agent
	srvrecords []string
	logger     *log.Logger
	replay     bool
}

// NewAgentSRV is used to create a new AgentSRV
func NewAgentSRV(agent *Agent, logOutput io.Writer, replay bool, srvrecords []string) (*AgentSRV, error) {

	// Initialize the AgentSRV
	m := &AgentSRV{
		agent:      agent,
		srvrecords: srvrecords,
		logger:     log.New(logOutput, "", log.LstdFlags),
		replay:     replay,
	}

	// Start the poller in the background
	go m.run()
	return m, nil
}

// run is a long running goroutine that scans for new hosts periodically
func (m *AgentSRV) run() {

	for {
		// Format the host address
		records := m.querySRV()

		n, err := m.agent.Join(records, m.replay)

		// Attempt the join
		if err != nil {
			m.logger.Printf("[ERR] agent.srv: Failed to join: %v", err)
		}
		if n > 0 {
			m.logger.Printf("[INFO] agent.srv: Joined %d hosts", n)
		}

		time.Sleep(srvPollInterval)
	}
}

// querySRV looks up the SRV records and returns a slice of all SRV records
func (m *AgentSRV) querySRV() []string {
	var hosts []string
	for _, record := range m.srvrecords {
		_, srvhosts, err := net.LookupSRV("", "", record)

		if err != nil {
			m.logger.Printf("[ERR] agent.srv: Failed to poll for new hosts: %v", err)
		}

		for _, host := range srvhosts {
			addr := fmt.Sprintf("%s:%d", host.Target, host.Port)
			hosts = append(hosts, addr)
		}
	}
	return hosts
}
