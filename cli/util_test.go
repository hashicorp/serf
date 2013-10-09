package cli

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

func yield() {
	time.Sleep(10 * time.Millisecond)
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
