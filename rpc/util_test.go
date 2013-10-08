package rpc

import (
	"net"
	"sync"
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

func yield() {
	time.Sleep(5 * time.Millisecond)
}
