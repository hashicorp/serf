package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"math/rand"
	"net"
	"time"
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

func getRPCAddr() string {
	for i := 0; i < 500; i++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", rand.Int31n(25000)+1024))
		if err == nil {
			l.Close()
			return l.Addr().String()
		}
	}

	panic("no listener")
}

func testAgent() *Agent {
	config := serf.DefaultConfig()
	config.MemberlistConfig.BindAddr = testutil.GetBindAddr().String()
	config.NodeName = config.MemberlistConfig.BindAddr

	agent := &Agent{
		RPCAddr:    getRPCAddr(),
		SerfConfig: config,
	}

	return agent
}
