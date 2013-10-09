package serf

import (
	"github.com/hashicorp/memberlist"
	"testing"
)

func TestSerf_joinLeave_ltime(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	if s2.members[s1.config.NodeName].joinLTime != 1 {
		t.Fatalf("join time is not valid %d",
			s2.members[s1.config.NodeName].joinLTime)
	}

	if s2.clock.Time() <= s2.members[s1.config.NodeName].joinLTime {
		t.Fatalf("join should increment")
	}
	oldClock := s2.clock.Time()

	err = s1.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	// s1 clock should exceed s2 due to leave
	if s2.clock.Time() <= oldClock {
		t.Fatalf("leave should increment (%d / %d)",
			s2.clock.Time(), oldClock)
	}
}

func TestSerf_join_pendingIntents(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.recentJoin[0] = nodeIntent{5, "test"}
	s.recentLeave[0] = nodeIntent{6, "test"}

	n := memberlist.Node{Name: "test",
		Addr: nil,
		Meta: []byte("test"),
	}

	s.handleNodeJoin(&n)

	mem := s.members["test"]
	if mem.joinLTime != 5 {
		t.Fatalf("bad join time")
	}
	if mem.Status != StatusLeaving {
		t.Fatalf("bad status")
	}
}

func TestSerf_forceRemove_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusFailed,
		},
		joinLTime: 12,
	}

	r := messageRemoveFailed{LTime: 10, Node: "test"}
	if s.handleNodeForceRemove(&r) {
		t.Fatalf("should not rebroadcast")
	}

	if s.members["test"].Status != StatusFailed {
		t.Fatalf("should still be failed")
	}
}

func TestSerf_forceRemove_noNode(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	r := messageRemoveFailed{LTime: 10, Node: "test"}
	if s.handleNodeForceRemove(&r) {
		t.Fatalf("should not rebroadcast")
	}

	if s.clock.Time() != 11 {
		t.Fatalf("time wrong")
	}
}

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
