package command

import (
	"fmt"
	"github.com/hashicorp/serf/command/agent"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"math/rand"
	"net"
	"testing"
	"time"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

func testAgent(t *testing.T) *agent.Agent {
	config := serf.DefaultConfig()
	config.MemberlistConfig.BindAddr = testutil.GetBindAddr().String()
	config.MemberlistConfig.ProbeInterval = 50 * time.Millisecond
	config.MemberlistConfig.ProbeTimeout = 25 * time.Millisecond
	config.MemberlistConfig.SuspicionMult = 1
	config.NodeName = config.MemberlistConfig.BindAddr

	agent := &agent.Agent{
		RPCAddr:    getRPCAddr(),
		SerfConfig: config,
	}

	if err := agent.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	return agent
}

func getRPCAddr() string {
	for i := 0; i < 500; i++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", rand.Int31n(25000)+1024))
		if err == nil {
			l.Close()
			return l.Addr().String()
		}
	}

	panic("no listener")
}
