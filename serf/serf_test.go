package serf

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func testConfig() *Config {
	config := DefaultConfig()
	config.MemberlistConfig.BindAddr = getBindAddr().String()

	// Set probe intervals that are aggressive for finding bad nodes
	config.MemberlistConfig.ProbeInterval = 50 * time.Millisecond
	config.MemberlistConfig.ProbeTimeout = 25 * time.Millisecond
	config.MemberlistConfig.SuspicionMult = 1

	config.NodeName = fmt.Sprintf("Node %s", config.MemberlistConfig.BindAddr)

	// Set a short reap interval so that it can run during the test
	config.ReapInterval = 1 * time.Second

	// Set a short reconnect interval so that it can run a lot during tests
	config.ReconnectInterval = 100 * time.Millisecond

	// Set basically zero on the reconnect/tombstone timeouts so that
	// they're removed on the first ReapInterval.
	config.ReconnectTimeout = 1 * time.Microsecond
	config.TombstoneTimeout = 1 * time.Microsecond

	return config
}

// testMember tests that a member in a list is in a given state.
func testMember(t *testing.T, members []Member, name string, status MemberStatus) {
	for _, m := range members {
		if m.Name == name {
			if m.Status != status {
				t.Fatalf("bad state for %s: %d", name, m.Status)
			}

			return
		}
	}

	if status == StatusNone {
		// We didn't expect to find it
		return
	}

	t.Fatalf("node not found: %s", name)
}

func yield() {
	time.Sleep(5 * time.Millisecond)
}

func TestSerfCreate_noName(t *testing.T) {
	t.Parallel()

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	config := testConfig()
	config.NodeName = ""

	_, err = Create(config)
	if err != nil {
		t.Fatalf("should not have error")
	}

	if config.NodeName != hostname {
		t.Fatalf("bad node name: %s", config.NodeName)
	}
}

func TestSerf_eventsFailed(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(1 * time.Second)

	// Since s2 shutdown, we check the events to make sure we got failures.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberFailed})
}

func TestSerf_eventsJoin(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin})
}

func TestSerf_eventsLeave(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	// Now that s2 has left, we check the events to make sure we got
	// a leave event in s1 about the leave.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberLeave})
}

func TestSerf_joinLeave(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	yield()

	if len(s1.Members()) != 1 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 1 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	if len(s1.Members()) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 2 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	err = s1.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Give the reaper time to reap nodes
	time.Sleep(s1Config.ReapInterval * 2)

	if len(s1.Members()) != 1 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 1 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}
}

func TestSerf_reconnect(t *testing.T) {
	eventCh := make(chan Event, 64)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()
	s2Addr := s2Config.MemberlistConfig.BindAddr
	s2Name := s2Config.NodeName

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Bring back s2 by mimicking its name and address
	s2Config = testConfig()
	s2Config.MemberlistConfig.BindAddr = s2Addr
	s2Config.NodeName = s2Name
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s1Config.ReconnectInterval * 5)

	testEvents(t, eventCh, s2Name,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberJoin})
}

// internals
func TestSerf_resetLeaveIntent(t *testing.T) {
	s1Config := testConfig()
	s1Config.LeaveTimeout = 10 * time.Millisecond

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	yield()

	s1.handleNodeLeaveIntent(&messageLeave{
		Node: s1Config.NodeName,
	})

	members := s1.Members()
	if members[0].Status != StatusLeaving {
		t.Fatalf("status should be leaving: %d", members[0].Status)
	}

	time.Sleep(s1Config.LeaveTimeout + 10*time.Millisecond)

	members = s1.Members()
	if members[0].Status == StatusLeaving {
		t.Fatalf("status should not be leaving: %d", members[0].Status)
	}
}

func TestSerf_role(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()

	s1Config.Role = "web"
	s2Config.Role = "lb"

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	members := s1.Members()
	if len(members) != 2 {
		t.Fatalf("should have 2 members")
	}

	roles := make(map[string]string)
	for _, m := range members {
		roles[m.Name] = m.Role
	}

	if roles[s1Config.NodeName] != "web" {
		t.Fatalf("bad role for web: %s", roles[s1Config.NodeName])
	}

	if roles[s2Config.NodeName] != "lb" {
		t.Fatalf("bad role for lb: %s", roles[s2Config.NodeName])
	}
}

func TestSerfRemoveFailedNode(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()
	s3Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()
	defer s3.Shutdown()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err = s1.Join([]string{s3Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Verify that s2 is "failed"
	testMember(t, s1.Members(), s2Config.NodeName, StatusFailed)

	// Now remove the failed node
	if err := s1.RemoveFailedNode(s2Config.NodeName); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that s2 is gone
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)
	testMember(t, s3.Members(), s2Config.NodeName, StatusLeft)
}

func TestSerfState(t *testing.T) {
	s1, err := Create(testConfig())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	if s1.State() != SerfAlive {
		t.Fatalf("bad state: %d", s1.State())
	}

	if err := s1.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if s1.State() != SerfLeft {
		t.Fatalf("bad state: %d", s1.State())
	}

	if err := s1.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if s1.State() != SerfShutdown {
		t.Fatalf("bad state: %d", s1.State())
	}
}

func TestSerf_ReapHandler_Shutdown(t *testing.T) {
	s, err := Create(testConfig())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	go func() {
		s.Shutdown()
		time.Sleep(time.Millisecond)
		t.Fatalf("timeout")
	}()
	s.handleReap()
}

func TestSerf_ReapHandler(t *testing.T) {
	c := testConfig()
	c.ReapInterval = time.Nanosecond
	c.TombstoneTimeout = time.Second * 6
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	m := &Member{}
	s.leftMembers = []*oldMember{
		&oldMember{m, time.Now()},
		&oldMember{m, time.Now().Add(-5 * time.Second)},
		&oldMember{m, time.Now().Add(-10 * time.Second)},
	}

	go func() {
		time.Sleep(time.Millisecond)
		s.Shutdown()
	}()

	s.handleReap()

	if len(s.leftMembers) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestSerf_Reap(t *testing.T) {
	s, err := Create(testConfig())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	m := &Member{}
	old := []*oldMember{
		&oldMember{m, time.Now()},
		&oldMember{m, time.Now().Add(-5 * time.Second)},
		&oldMember{m, time.Now().Add(-10 * time.Second)},
	}

	old = s.reap(old, time.Second*6)
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestRemoveOldMember(t *testing.T) {
	old := []*oldMember{
		&oldMember{member: &Member{Name: "foo"}},
		&oldMember{member: &Member{Name: "bar"}},
		&oldMember{member: &Member{Name: "baz"}},
	}

	old = removeOldMember(old, "bar")
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
	if old[1].member.Name == "bar" {
		t.Fatalf("should remove old member")
	}
}
