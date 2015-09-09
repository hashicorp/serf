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
		records := m.findSRV()

		// Attempt the join only if there are new records
		if len(records) > 0 {
			n, err := m.agent.Join(records, m.replay)

			if err != nil {
				m.logger.Printf("[ERR] agent.srv: Failed to join: %v", err)
			}
			if n > 0 {
				m.logger.Printf("[INFO] agent.srv: Joined %d hosts", n)
			}
		}

		// Sleep until it's time to poll again
		time.Sleep(srvPollInterval)
	}
}

// findSRV looks up the SRV records and returns a slice of all SRV records
// that are not currently cluster members
func (m *AgentSRV) findSRV() []string {
	var hosts []string

	// map the members so that we only do a single O(n) search through members
	known_members := make(map[string]bool)
	for _, v := range m.agent.Serf().Members() {
		member := fmt.Sprintf("%s:%d", v.Addr.String(), v.Port)
		known_members[member] = true
	}

	// Look up each SRV record and check if it's already in the cluster
	for _, record := range m.srvrecords {
		_, srvhosts, err := net.LookupSRV("", "", record)

		if err != nil {
			m.logger.Printf("[ERR] agent.srv: Failed to poll for new hosts: %v", err)
		}

		// Filter each hosts in the SRV record
		for _, host := range srvhosts {
			// Find its addresses, as it's the only unique ID we can rely on
			ipaddrs, err := net.LookupIP(host.Target)

			if err != nil {
				m.logger.Printf("[ERR] agent.srv: resolve SRV record %s to IP %v", host.Target, err)
			}

			// For each address the host has, check if it's already in the cluster
			for _, ipaddr := range ipaddrs {
				addr := fmt.Sprintf("%s:%d", ipaddr.String(), host.Port)
				if _, known := known_members[addr]; !known {
					// If the host is not already in the cluster,
					// Add it to the list of hosts to try to join
					hosts = append(hosts, addr)
				}
			}
		}
	}
	return hosts
}
