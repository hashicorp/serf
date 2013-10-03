package serf

import (
	"testing"
	"time"
)

func TestNoopDetector(t *testing.T) {
	n := noopDetector{}
	m := &Member{}
	n.Suspect(m)
	n.Unsuspect(m)
	if n.PartitionDetected() {
		t.Fatalf("unexpected partition")
	}
	if n.PartitionedMembers() != nil {
		t.Fatalf("unexpected members")
	}
}

func TestPartitionRing_Suspect(t *testing.T) {
	m1 := &Member{}
	m2 := &Member{}
	m3 := &Member{}
	m4 := &Member{}
	p := newPartitionRing(4, time.Second)

	p.Suspect(m1)
	if p.PartitionDetected() {
		t.Fatalf("unexpected partition")
	}

	p.Suspect(m2)
	if p.PartitionDetected() {
		t.Fatalf("unexpected partition")
	}

	p.Suspect(m3)
	if p.PartitionDetected() {
		t.Fatalf("unexpected partition")
	}

	p.Suspect(m4)
	if !p.PartitionDetected() {
		t.Fatalf("expected partition")
	}

	mems := p.PartitionedMembers()
	if len(mems) != 4 {
		t.Fatalf("should be 4 memebers")
	}
}

func TestPartitionRing_Unsuspect(t *testing.T) {
	m1 := &Member{}
	m2 := &Member{}
	m3 := &Member{}
	m4 := &Member{}
	p := newPartitionRing(4, time.Second)

	// Not yet suspected, but shouldn't fail
	p.Unsuspect(m1)

	p.Suspect(m1)
	p.Suspect(m2)
	p.Suspect(m3)
	p.Suspect(m4)

	// Unsuspect a node
	p.Unsuspect(m3)

	// Should no longer detect
	if p.PartitionDetected() {
		t.Fatalf("unexpected partition")
	}
}

func TestPartitionRing_OldSuspect(t *testing.T) {
	m1 := &Member{}
	m2 := &Member{}
	m3 := &Member{}
	m4 := &Member{}
	p := newPartitionRing(4, time.Second)

	p.Suspect(m1)
	p.Suspect(m2)
	p.Suspect(m3)
	p.Suspect(m4)

	// Change the suspect time
	p.ring[0].failTime = time.Now().Add(-2 * time.Second)

	// Should no longer detect
	if p.PartitionDetected() {
		t.Fatalf("unexpected partition")
	}
}

func TestSerf_SuspectPartition(t *testing.T) {
	ch := make(chan statusChange, 4)
	s := &Serf{changeCh: ch}
	s.detector = newPartitionRing(4, time.Second)

	m1 := &Member{Status: StatusFailed}
	m2 := &Member{Status: StatusFailed}
	m3 := &Member{Status: StatusFailed}
	m4 := &Member{Status: StatusFailed}

	s.suspectPartition(m1)
	s.suspectPartition(m2)
	s.suspectPartition(m3)
	s.suspectPartition(m4)

	if len(ch) != 4 {
		t.Fatalf("expected 4 status changes")
	}
	for i := 0; i < 4; i++ {
		change := <-ch
		if change.newStatus != StatusPartitioned {
			t.Fatalf("Expected partitioned status")
		}
		if change.member.Status != StatusPartitioned {
			t.Fatalf("Expected partitioned status")
		}
	}
}
