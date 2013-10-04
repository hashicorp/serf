package serf

import (
	"net"
	"time"
)

// reconnectHandler is a long running routine that attempts to
// reconnect to nodes that have failed or been partitioned. It allows
// Serf to automatically recover from network partitions.
func (s *Serf) reconnectHandler() {
	for {
		select {
		case <-time.After(s.conf.ReconnectInterval):
			s.attemptReconnect()
		case <-s.shutdownCh:
			return
		}
	}
}

// attemptReconnect is called to attempt to reconnect to a single
// previously failed node
func (s *Serf) attemptReconnect() {
	s.memberLock.RLock()

	// Nothing to do if there are no failed members
	n := len(s.failedMembers)
	if n == 0 {
		s.memberLock.RUnlock()
		return
	}

	// Select a random member to try and join
	idx := randomOffset(n)
	mem := s.failedMembers[idx]
	s.memberLock.RUnlock()

	// Format the addr
	addr := net.IP(mem.member.Addr).String()

	// Attempt to join at the memberlist level
	s.memberlist.Join([]string{addr})
}
