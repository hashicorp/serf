package cli

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

var bindLock sync.Mutex
var bindNum byte = 10

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

func yield() {
	time.Sleep(10 * time.Millisecond)
}

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

func getRPCAddr() string {
	for i := 0; i < 500; i++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", rand.Int31n(25000)+1024))
		if err == nil {
			l.Close()
			time.Sleep(1 * time.Second)
			return l.Addr().String()
		}
	}

	panic("no listener")
}
