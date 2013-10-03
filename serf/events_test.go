package serf

import (
	"testing"
)

func TestPartitionEvents(t *testing.T) {
	m1 := &Member{}
	m2 := &Member{}
	m3 := &Member{}
	m4 := &Member{}
	m5 := &Member{}

	init := map[*Member]MemberStatus{
		m1: StatusNone,
		m2: StatusAlive,
		m3: StatusFailed,
		m4: StatusLeaving,
		m5: StatusAlive,
	}
	end := map[*Member]MemberStatus{
		m1: StatusAlive,
		m2: StatusFailed,
		m3: StatusPartitioned,
		m4: StatusLeft,
		m5: StatusAlive,
	}

	joined, left, failed, partitioned := partitionEvents(init, end)

	if len(joined) != 1 || joined[0] != m1 {
		t.Fatalf("m1 should have joined!")
	}
	if len(left) != 1 || left[0] != m4 {
		t.Fatalf("m4 should have left!")
	}
	if len(failed) != 1 || failed[0] != m2 {
		t.Fatalf("m2 should have failed!")
	}
	if len(partitioned) != 1 || partitioned[0] != m3 {
		t.Fatalf("m3 should have partitioned!")
	}
}
