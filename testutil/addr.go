// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"container/list"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

var (
	bindLock     sync.Mutex
	freeIPs      *list.List
	condNotEmpty *sync.Cond
)

const bindLockPort = 10101

func init() {
	freeIPs = list.New()
	condNotEmpty = sync.NewCond(&bindLock)
	for octet := byte(10); octet < 255; octet++ {
		result := net.IPv4(127, 0, 0, octet)
		freeIPs.PushBack(result)
	}
}

func returnIP(ip net.IP) {
	bindLock.Lock()
	defer bindLock.Unlock()
	freeIPs.PushBack(ip)
	condNotEmpty.Broadcast()
}

func getBindAddr() net.IP {
	bindLock.Lock()
	defer bindLock.Unlock()

	for freeIPs.Len() == 0 {
		condNotEmpty.Wait()
	}

	elem := freeIPs.Front()
	freeIPs.Remove(elem)
	result := elem.Value.(net.IP)

	return result
}

func TakeIP() (ip net.IP, returnFn func()) {
	for attempts := 0; ; attempts++ {
		ip = getBindAddr()

		addr := &net.TCPAddr{IP: ip, Port: bindLockPort}

		ln, err := net.ListenTCP("tcp4", addr)
		if err != nil {
			returnIP(ip)
			continue
		}

		if attempts > 3 {
			logf("took %s after %d attempts", ip, attempts)
		}
		return ip, func() {
			ln.Close()
			time.Sleep(50 * time.Millisecond) // let the kernel cool down
			returnIP(ip)
		}
	}
}

func logf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, "testutil: "+format+"\n", a...)
}
