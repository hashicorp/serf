// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	mdnsPollInterval  = 60 * time.Second
	mdnsQuietInterval = 100 * time.Millisecond
)

// AgentMDNS is used to advertise ourself using mDNS and to
// attempt to join peers periodically using mDNS queries.
type AgentMDNS struct {
	agent    *Agent
	discover string
	logger   *slog.Logger
	seen     map[string]struct{}
	server   *mdns.Server
	replay   bool
	iface    *net.Interface
}

// NewAgentMDNS is used to create a new AgentMDNS
func NewAgentMDNS(agent *Agent, logOutput io.Writer, replay bool,
	node, discover string, iface *net.Interface, bind net.IP, port int) (*AgentMDNS, error) {
	// Create the service
	service, err := mdns.NewMDNSService(
		node,
		mdnsName(discover),
		"",
		"",
		port,
		[]net.IP{bind},
		[]string{fmt.Sprintf("Serf '%s' cluster", discover)})
	if err != nil {
		return nil, err
	}

	// Configure mdns server
	conf := &mdns.Config{
		Zone:  service,
		Iface: iface,
	}

	// Create the server
	server, err := mdns.NewServer(conf)
	if err != nil {
		return nil, err
	}

	var logLevel *slog.Level
	if err := logLevel.UnmarshalText([]byte(agent.agentConf.LogLevel)); err != nil {
		return nil, err
	}
	handlerOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	handler := slog.NewTextHandler(os.Stdout, handlerOpts)

	// Initialize the AgentMDNS
	m := &AgentMDNS{
		agent:    agent,
		discover: discover,
		logger:   slog.New(handler).WithGroup("agent.mdns"),
		seen:     make(map[string]struct{}),
		server:   server,
		replay:   replay,
		iface:    iface,
	}

	// Start the background workers
	go m.run()
	return m, nil
}

// run is a long running goroutine that scans for new hosts periodically
func (m *AgentMDNS) run() {
	hosts := make(chan *mdns.ServiceEntry, 32)
	poll := time.After(0)
	var quiet <-chan time.Time
	var join []string

	for {
		select {
		case h := <-hosts:
			// Format the host address
			addr := net.TCPAddr{IP: h.Addr, Port: h.Port}
			addrS := addr.String()

			// Skip if we've handled this host already
			if _, ok := m.seen[addrS]; ok {
				continue
			}

			// Queue for handling
			join = append(join, addrS)
			quiet = time.After(mdnsQuietInterval)

		case <-quiet:
			// Attempt the join
			n, err := m.agent.Join(join, m.replay)
			if err != nil {
				m.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to join", slog.String("error", err.Error()))
			}
			if n > 0 {
				m.logger.LogAttrs(context.TODO(), slog.LevelInfo, "Joined hosts", slog.Int("count", n))
			}

			// Mark all as seen
			for _, n := range join {
				m.seen[n] = struct{}{}
			}
			join = nil

		case <-poll:
			poll = time.After(mdnsPollInterval)
			go m.poll(hosts)
		}
	}
}

// poll is invoked periodically to check for new hosts
func (m *AgentMDNS) poll(hosts chan *mdns.ServiceEntry) {
	params := mdns.QueryParam{
		Service:   mdnsName(m.discover),
		Interface: m.iface,
		Entries:   hosts,
	}
	if err := mdns.Query(&params); err != nil {
		m.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to poll for new hosts", slog.String("error", err.Error()))
	}
}

// mdnsName returns the service name to register and to lookup
func mdnsName(discover string) string {
	return fmt.Sprintf("_serf_%s._tcp", discover)
}
