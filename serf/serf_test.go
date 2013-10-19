package serf

import (
	"fmt"
	"github.com/hashicorp/serf/testutil"
	"strings"
	"testing"
	"time"
)

func testConfig() *Config {
	config := DefaultConfig()
	config.MemberlistConfig.BindAddr = testutil.GetBindAddr().String()

	// Set probe intervals that are aggressive for finding bad nodes
	config.MemberlistConfig.GossipInterval = 5 * time.Millisecond
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

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

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

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

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

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Now that s2 has left, we check the events to make sure we got
	// a leave event in s1 about the leave.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberLeave})
}

func TestSerf_eventsUser(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.EventCh = eventCh

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

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test")); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar")); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// check the events to make sure we got
	// a leave event in s1 about the leave.
	testUserEvents(t, eventCh,
		[]string{"event!", "second"},
		[][]byte{[]byte("test"), []byte("foobar")})
}

func TestSerf_eventsUser_sizeLimit(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	name := "this is too large an event"
	payload := make([]byte, UserEventSizeLimit)
	err = s1.UserEvent(name, payload)
	if strings.HasPrefix(err.Error(), "user event exceeds limit of ") {
		t.Fatalf("should get size limit error")
	}
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

	testutil.Yield()

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

	testutil.Yield()

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

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

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

	testutil.Yield()

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

	testutil.Yield()

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

	m := Member{}
	s.leftMembers = []*memberState{
		&memberState{m, 0, time.Now()},
		&memberState{m, 0, time.Now().Add(-5 * time.Second)},
		&memberState{m, 0, time.Now().Add(-10 * time.Second)},
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

	m := Member{}
	old := []*memberState{
		&memberState{m, 0, time.Now()},
		&memberState{m, 0, time.Now().Add(-5 * time.Second)},
		&memberState{m, 0, time.Now().Add(-10 * time.Second)},
	}

	old = s.reap(old, time.Second*6)
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestRemoveOldMember(t *testing.T) {
	old := []*memberState{
		&memberState{Member: Member{Name: "foo"}},
		&memberState{Member: Member{Name: "bar"}},
		&memberState{Member: Member{Name: "baz"}},
	}

	old = removeOldMember(old, "bar")
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
	if old[1].Name == "bar" {
		t.Fatalf("should remove old member")
	}
}

func TestRecentIntent(t *testing.T) {
	if recentIntent(nil, "foo") != nil {
		t.Fatalf("should get nil on empty recent")
	}
	if recentIntent([]nodeIntent{}, "foo") != nil {
		t.Fatalf("should get nil on empty recent")
	}

	recent := []nodeIntent{
		nodeIntent{1, "foo"},
		nodeIntent{2, "bar"},
		nodeIntent{3, "baz"},
		nodeIntent{4, "bar"},
		nodeIntent{0, "bar"},
		nodeIntent{5, "bar"},
	}

	if r := recentIntent(recent, "bar"); r.LTime != 4 {
		t.Fatalf("bad time for bar")
	}

	if r := recentIntent(recent, "tubez"); r != nil {
		t.Fatalf("got result for tubez")
	}
}

func TestMemberStatus_String(t *testing.T) {
	status := []MemberStatus{StatusNone, StatusAlive, StatusLeaving, StatusLeft, StatusFailed}
	expect := []string{"none", "alive", "leaving", "left", "failed"}

	for idx, s := range status {
		if s.String() != expect[idx] {
			t.Fatalf("got string %v, expected %v", s.String(), expect[idx])
		}
	}

	other := MemberStatus(100)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	other.String()
}
