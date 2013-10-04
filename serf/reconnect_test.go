package serf

import (
	"testing"
	"time"
)

func TestSerf_ReconnectHandler_Shutdown(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	close(s.shutdownCh)

	go func() {
		time.Sleep(time.Millisecond)
		t.Fatalf("timeout")
	}()

	s.reconnectHandler()
}

func TestSerf_ReconnectHandler(t *testing.T) {
	s := GetSerf(t)
	s.conf.ReconnectInterval = time.Nanosecond

	// Artificially failed node
	s.memberLock.Lock()
	m := &Member{Addr: []byte{127, 0, 0, 1}}
	s.failedMembers = []*oldMember{
		&oldMember{m, time.Now()},
	}
	s.memberLock.Unlock()

	go func() {
		time.Sleep(time.Millisecond)
		s.Shutdown()
	}()

	s.reconnectHandler()
}
