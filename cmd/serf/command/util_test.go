// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"io"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

func testAgent(t *testing.T, ip net.IP) *agent.Agent {
	agentConfig := agent.DefaultConfig()
	serfConfig := serf.DefaultConfig()
	return testAgentWithConfig(t, ip, agentConfig, serfConfig)
}

func testAgentWithConfig(t *testing.T, ip net.IP, agentConfig *agent.Config, serfConfig *serf.Config) *agent.Agent {
	serfConfig.MemberlistConfig.BindAddr = ip.String()
	serfConfig.MemberlistConfig.ProbeInterval = 50 * time.Millisecond
	serfConfig.MemberlistConfig.ProbeTimeout = 25 * time.Millisecond
	serfConfig.MemberlistConfig.SuspicionMult = 1
	serfConfig.NodeName = serfConfig.MemberlistConfig.BindAddr
	serfConfig.Tags = map[string]string{"role": "test", "tag1": "foo", "tag2": "bar"}

	serfConfig.MemberlistConfig.RequireNodeNames = true

	agent, err := agent.Create(agentConfig, serfConfig, testutil.TestWriter(t))
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := agent.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}

	return agent
}

func testIPC(t *testing.T, ip net.IP, a *agent.Agent) (string, *agent.AgentIPC) {
	rpcAddr := ip.String() + ":11111"

	l, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	tw := testutil.TestWriter(t)

	lw := agent.NewLogWriter(512)
	mult := io.MultiWriter(tw, lw)
	ipc := agent.NewAgentIPC(a, "", l, mult, lw, false)
	return rpcAddr, ipc
}
