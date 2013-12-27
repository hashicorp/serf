package serf

import (
	"fmt"
	"github.com/hashicorp/serf/testutil"
	"io/ioutil"
	"os"
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

func TestCreate_badProtocolVersion(t *testing.T) {
	cases := []struct {
		version uint8
		err     bool
	}{
		{ProtocolVersionMin, false},
		{ProtocolVersionMax, false},
		// TODO(mitchellh): uncommon when we're over 0
		//{ProtocolVersionMin - 1, true},
		{ProtocolVersionMax + 1, true},
		{ProtocolVersionMax - 1, false},
	}

	for _, tc := range cases {
		c := testConfig()
		c.ProtocolVersion = tc.version
		s, err := Create(c)
		if tc.err && err == nil {
			t.Errorf("Should've failed with version: %d", tc.version)
		} else if !tc.err && err != nil {
			t.Errorf("Version '%d' error: %s", tc.version, err)
		}

		if err == nil {
			s.Shutdown()
		}
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

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar"), false); err != nil {
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
	err = s1.UserEvent(name, payload, false)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

// Bug: GH-58
func TestSerf_leaveRejoinDifferentRole(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.Role = "foo"

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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	err = s2.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Make s3 look just like s2, but create a new node with a new role
	s3Config := testConfig()
	s3Config.MemberlistConfig.BindAddr = s2Config.MemberlistConfig.BindAddr
	s3Config.NodeName = s2Config.NodeName
	s3Config.Role = "bar"

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s3.Shutdown()

	_, err = s3.Join([]string{s1Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	members := s1.Members()
	if len(members) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	var member *Member = nil
	for _, m := range members {
		if m.Name == s3Config.NodeName {
			member = &m
			break
		}
	}

	if member == nil {
		t.Fatalf("couldn't find member")
	}

	if member.Role != s3Config.Role {
		t.Fatalf("bad role: %s", member.Role)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

func TestSerfProtocolVersion(t *testing.T) {
	config := testConfig()
	config.ProtocolVersion = ProtocolVersionMax

	s1, err := Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	actual := s1.ProtocolVersion()
	if actual != ProtocolVersionMax {
		t.Fatalf("bad: %#v", actual)
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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err = s1.Join([]string{s3Config.MemberlistConfig.BindAddr}, false)
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

func TestSerfRemoveFailedNode_ourself(t *testing.T) {
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	testutil.Yield()

	if err := s1.RemoveFailedNode("somebody"); err != nil {
		t.Fatalf("err: %s", err)
	}
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

func TestSerf_joinLeaveJoin(t *testing.T) {
	s1Config := testConfig()
	s1Config.ReapInterval = 10 * time.Second
	s2Config := testConfig()
	s2Config.ReapInterval = 10 * time.Second

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if len(s1.Members()) != 1 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 1 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
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

	// Leave and shutdown
	err = s2.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s2.Shutdown()

	// Give the reaper time to reap nodes
	time.Sleep(s1Config.MemberlistConfig.ProbeInterval * 5)

	// s1 should see the node as having left
	mems := s1.Members()
	anyLeft := false
	for _, m := range mems {
		if m.Status == StatusLeft {
			anyLeft = true
			break
		}
	}
	if !anyLeft {
		t.Fatalf("node should have left!")
	}

	// Bring node 2 back
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	testutil.Yield()

	// Re-attempt the join
	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Should be back to both members
	if len(s1.Members()) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 2 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	// s1 should see the node as alive
	mems = s1.Members()
	anyLeft = false
	for _, m := range mems {
		if m.Status == StatusLeft {
			anyLeft = true
			break
		}
	}
	if anyLeft {
		t.Fatalf("all nodes should be alive!")
	}
}

func TestSerf_Join_IgnoreOld(t *testing.T) {
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

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// join with ignoreOld set to true! should not get events
	_, err = s2.Join([]string{s1Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// check the events to make sure we got nothing
	testUserEvents(t, eventCh, []string{}, [][]byte{})
}

func TestSerf_SnapshotRecovery(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.SnapshotPath = td + "snap"

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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
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

	// Listen for events
	eventCh := make(chan Event, 4)
	s2Config.EventCh = eventCh

	// Restart s2 from the snapshot now!
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	// Wait for the node to auto rejoin
	testutil.Yield()
	testutil.Yield()
	testutil.Yield()

	// Verify that s2 is "alive"
	testMember(t, s1.Members(), s2Config.NodeName, StatusAlive)
	testMember(t, s2.Members(), s1Config.NodeName, StatusAlive)

	// Check the events to make sure we got nothing
	testUserEvents(t, eventCh, []string{}, [][]byte{})
}

func TestSerf_Leave_SnapshotRecovery(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.SnapshotPath = td + "snap"

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

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}
	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Verify that s2 is "left"
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)

	// Restart s2 from the snapshot now!
	s2Config.EventCh = nil
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	// Wait for the node to auto rejoin
	testutil.Yield()

	// Verify that s2 is didn't join
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)
	if len(s2.Members()) != 1 {
		t.Fatalf("bad members: %#v", s2.Members())
	}
}
