package serf

import (
	"time"
)

// partitionDetector interface is used to provide pluggable implementations
// of parition detection heuristics
type partitionDetector interface {
	Suspect(*Member)
	Unsuspect(*Member)
	PartitionDetected() bool
	PartitionedMembers() []*Member
}

// partitionRing maintains a circular ring of failure events, that
// could potentially signal a network partition has taken place. The heuristic
// for a partition is if X failures occur in Y time. e.g. 2 failures in 10 seconds.
// We simply use a ring of size X, and check if all events are within Y time.
type partitionRing struct {
	index     int
	ring      []*memberFailure
	threshold time.Duration
}

// noopDetector is a heuristic that never detects a partition. It can be used
// when partition detection should be disabled.
type noopDetector struct{}

// memberFailure tracks the failure of a specific member
type memberFailure struct {
	failTime time.Time // Time failure took place
	member   *Member   // Member it reflects
}

// suspectPartition is called when a member fails, and we may suspect
// a network partition. This method MUST be called with the memberLock held.
func (s *Serf) suspectPartition(mem *Member) {
	// Suspect a partition
	s.detector.Suspect(mem)

	// Check if the detector believes we are in a partitioned state
	if !s.detector.PartitionDetected() {
		return
	}

	// Get the paritioned nodes
	partitioned := s.detector.PartitionedMembers()

	// Update the status and notify of the change
	for _, mem := range partitioned {
		oldStatus := mem.Status
		if oldStatus == StatusFailed {
			mem.Status = StatusPartitioned
			s.changeCh <- statusChange{mem, oldStatus, StatusPartitioned}
		}
	}
}

// unsuspectPartition is called when a previously failed member comes back
// alive. This method MUST be called with the memberLock held.
func (s *Serf) unsuspectPartition(mem *Member) {
	// Pass through to the detector
	s.detector.Unsuspect(mem)
}

// newParitionRing creates a paritionRing that triggers after num failures
// in threshold time take place.
func newPartitionRing(num int, threshold time.Duration) *partitionRing {
	p := &partitionRing{index: 0, ring: make([]*memberFailure, num), threshold: threshold}
	return p
}

// Suspect adds a new failure to the ring
func (p *partitionRing) Suspect(mem *Member) {
	p.ring[p.index] = &memberFailure{time.Now(), mem}
	p.index = (p.index + 1) % len(p.ring)
}

// Unsuspect marks all previous failures for a node as recovered
func (p *partitionRing) Unsuspect(mem *Member) {
	for idx := range p.ring {
		if p.ring[idx] != nil && p.ring[idx].member == mem {
			p.ring[idx] = nil
			return
		}
	}
}

// PartitionDetected returns if the heuristic for a partition is met
func (p *partitionRing) PartitionDetected() bool {
	minTime := time.Now().Add(-p.threshold)
	for _, memFail := range p.ring {
		if memFail == nil {
			return false
		}
		if memFail.failTime.Before(minTime) {
			return false
		}
	}
	return true
}

// PartitionedMembers is invoked after a partition is detected to
// get the affected memebers
func (p *partitionRing) PartitionedMembers() []*Member {
	partitioned := make([]*Member, 0, len(p.ring))
	for _, failed := range p.ring {
		partitioned = append(partitioned, failed.member)
	}
	return partitioned
}

// Implement the interface methods for the noopDetector
func (n noopDetector) Suspect(*Member)               {}
func (n noopDetector) Unsuspect(*Member)             {}
func (n noopDetector) PartitionDetected() bool       { return false }
func (n noopDetector) PartitionedMembers() []*Member { return nil }
