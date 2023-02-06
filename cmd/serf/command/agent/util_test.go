// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"io"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

func drainEventCh(ch <-chan string) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func testAgent(t *testing.T, ip net.IP, logOutput io.Writer) *Agent {
	return testAgentWithConfig(t, ip, DefaultConfig(), serf.DefaultConfig(), logOutput)
}

func testAgentWithConfig(t *testing.T, ip net.IP, agentConfig *Config, serfConfig *serf.Config, logOutput io.Writer) *Agent {
	serfConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	serfConfig.MemberlistConfig.BindAddr = ip.String()
	serfConfig.NodeName = serfConfig.MemberlistConfig.BindAddr

	// Activate the strictest version of memberlist validation to ensure
	// we properly pass node names through the serf layer.
	serfConfig.MemberlistConfig.RequireNodeNames = true

	if logOutput == nil {
		logOutput = testutil.TestWriter(t)
	}

	agent, err := Create(agentConfig, serfConfig, logOutput)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return agent
}
