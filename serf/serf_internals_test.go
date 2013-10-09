package serf

import (
	"testing"
)

func TestSerf_leaveIntent_bufferEarly(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	// Deliver a leave intent message early
	j := messageLeave{LTime: 10, Node: "test"}
	if !s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we buffered
	if s.recentLeaveIndex != 1 {
		t.Fatalf("bad index")
	}
	if s.recentLeave[0].Node != "test" || s.recentLeave[0].LTime != 10 {
		t.Fatalf("bad buffer")
	}
}

func TestSerf_leaveIntent_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusAlive,
		},
		joinLTime: 12,
	}

	j := messageLeave{LTime: 10, Node: "test"}
	if s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	if s.recentLeaveIndex != 0 {
		t.Fatalf("bad index")
	}
}

func TestSerf_leaveIntent_newer(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusAlive,
		},
		joinLTime: 12,
	}

	j := messageLeave{LTime: 14, Node: "test"}
	if !s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if s.recentLeaveIndex != 0 {
		t.Fatalf("bad index")
	}

	if s.members["test"].Status != StatusLeaving {
		t.Fatalf("should update status")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_joinIntent_bufferEarly(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	// Deliver a join intent message early
	j := messageJoin{LTime: 10, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleNodeJoinIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we buffered
	if s.recentJoinIndex != 1 {
		t.Fatalf("bad index")
	}
	if s.recentJoin[0].Node != "test" || s.recentJoin[0].LTime != 10 {
		t.Fatalf("bad buffer")
	}
}

func TestSerf_joinIntent_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		joinLTime: 12,
	}

	j := messageJoin{LTime: 10, Node: "test"}
	if s.handleNodeJoinIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	if s.recentJoinIndex != 0 {
		t.Fatalf("bad index")
	}
}

func TestSerf_joinIntent_newer(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		joinLTime: 12,
	}

	// Deliver a join intent message early
	j := messageJoin{LTime: 14, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if s.recentJoinIndex != 0 {
		t.Fatalf("bad index")
	}

	if s.members["test"].joinLTime != 14 {
		t.Fatalf("should update join time")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}
