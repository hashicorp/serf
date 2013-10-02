package serf

import (
	"github.com/hashicorp/memberlist"
)

// nodeJoin is fired when memberlist detects a node join
func (s *Serf) nodeJoin(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	// Check if we know about this node already
	mem, ok := s.memberMap[n.Name]
	if !ok {
		mem = &Member{
			Name:   n.Name,
			Addr:   n.Addr,
			Role:   string(n.Meta),
			Status: StatusAlive,
		}
		s.memberMap[n.Name] = mem
	} else {
		mem.Status = StatusAlive
	}

	// Notify about change
	s.changeCh <- mem
}

// nodeLeave is fired when memberlist detects a node join
func (s *Serf) nodeLeave(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	// Check if we know about this node
	mem, ok := s.memberMap[n.Name]
	if !ok {
		return
	}

	// Determine the state change
	switch mem.Status {
	case StatusAlive:
		mem.Status = StatusFailed
	case StatusLeaving:
		mem.Status = StatusLeft
	}

	// Check if we should notify about a change
	s.changeCh <- mem
}

// intendLeave is invoked when we get a message indicating
// an intention to leave. Returns true if we should re-broadcast
func (s *Serf) intendLeave(l *leave) bool {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	// Check if we know about this node
	mem, ok := s.memberMap[l.Node]
	if !ok {
		return false // unknown, don't rebroadcast
	}

	// If the node is currently alive, then mark as a pending leave
	// and re-broadcast
	if mem.Status == StatusAlive {
		mem.Status = StatusLeaving
		return true
	}

	// State update not relevant, ignore it
	return false
}
