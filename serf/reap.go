package serf

import (
	"time"
)

// oldMember is used to track members that are no longer active due
// to leaving, failing or paritioning
type oldMember struct {
	member *Member
	time   time.Time // Time of leaving / failure
}

// reapHandler is a long running routine that reaps tombstones for
// failed nodes as well as those that gracefully left
func (s *Serf) reapHandler() {
	for {
		select {
		case <-time.After(s.conf.ReapInterval):
			s.memberLock.Lock()
			s.failedMembers = s.reap(s.failedMembers, s.conf.ReconnectTimeout)
			s.leftMembers = s.reap(s.leftMembers, s.conf.TombstoneTimeout)
			s.memberLock.Unlock()
		case <-s.shutdownCh:
			return
		}
	}
}

// reap is called with a list of old members and timeout, and removes
// members that have exceeded the timeout. Safety is left to the caller
func (s *Serf) reap(old []*oldMember, timeout time.Duration) []*oldMember {
	now := time.Now()
	n := len(old)
	for i := 0; i < n; i++ {
		m := old[i]

		// Skip if the timeout is not yet reached
		if now.Sub(m.time) <= timeout {
			continue
		}

		// Delete from the list
		old[i], old[n-1] = old[n-1], nil
		old = old[:n-1]
		n--
		i--

		// Delete from members
		delete(s.members, m.member.Name)
	}
	return old
}

// removeOldMember is used to remove an old member from a list of
// old members. Safety of this is left to the caller.
func removeOldMember(old []*oldMember, m *Member) []*oldMember {
	for i := 0; i < len(old); i++ {
		if m == old[i].member {
			n := len(old)
			old[i], old[n-1] = old[n-1], nil
			return old[:n-1]
		}
	}
	return old
}
