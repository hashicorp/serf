package rpc

import (
	"github.com/hashicorp/serf/serf"
	"net"
	"sync"
	"testing"
	"time"
)

var bindLock sync.Mutex
var bindNum byte = 10

// Returns an unused address for binding to for tests.
func getBindAddr() net.IP {
	bindLock.Lock()
	defer bindLock.Unlock()

	result := net.IPv4(127, 0, 0, bindNum)
	bindNum++
	if bindNum > 255 {
		bindNum = 10
	}

	return result
}

func testSerf(t *testing.T) (*serf.Serf, string) {
	config := serf.DefaultConfig()
	config.MemberlistConfig.BindAddr = getBindAddr().String()
	config.NodeName = config.MemberlistConfig.BindAddr

	s, err := serf.Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return s, config.MemberlistConfig.BindAddr
}

func yield() {
	time.Sleep(5 * time.Millisecond)
}
