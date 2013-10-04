package serf

import (
	"github.com/hashicorp/memberlist"
	"reflect"
	"testing"
	"time"
)

type MockDelegate struct {
	joined      []*Member
	left        []*Member
	failed      []*Member
	partitioned []*Member
}

func (m *MockDelegate) MembersJoined(mems []*Member) {
	m.joined = mems
}

func (m *MockDelegate) MembersLeft(mems []*Member) {
	m.left = mems
}

func (m *MockDelegate) MembersFailed(mems []*Member) {
	m.failed = mems
}

func (m *MockDelegate) MembersPartitioned(mems []*Member) {
	m.partitioned = mems
}

type MockDetector struct {
	suspect   []*Member
	unsuspect []*Member
}

func (d *MockDetector) Suspect(m *Member) {
	d.suspect = append(d.suspect, m)
}

func (d *MockDetector) Unsuspect(m *Member) {
	d.unsuspect = append(d.unsuspect, m)
}
func (d *MockDetector) PartitionDetected() bool {
	return false
}
func (d *MockDetector) PartitionedMembers() []*Member {
	return nil
}

func TestSerf_ChangeHandler_Stop(t *testing.T) {
	go func() {
		time.Sleep(time.Second)
		t.Fatalf("timeout")
	}()

	s := &Serf{
		shutdownCh: make(chan struct{}),
	}
	close(s.shutdownCh)

	s.changeHandler()
}

func TestSerf_CoalesceUpdates_MaxTime(t *testing.T) {
	d := &MockDelegate{}
	c := &Config{
		MaxCoalesceTime:  time.Millisecond,
		MinQuiescentTime: time.Second,
		Delegate:         d,
	}
	s := &Serf{conf: c, changeCh: make(chan statusChange, 32)}

	m1 := &Member{}
	m2 := &Member{}

	// M1, none -> alive
	s.changeCh <- statusChange{m1, StatusNone, StatusAlive}
	s.changeCh <- statusChange{m1, StatusAlive, StatusFailed}
	s.changeCh <- statusChange{m1, StatusFailed, StatusPartitioned}
	s.changeCh <- statusChange{m1, StatusPartitioned, StatusAlive}

	// m2 alive -> alive
	s.changeCh <- statusChange{m2, StatusAlive, StatusFailed}
	s.changeCh <- statusChange{m2, StatusFailed, StatusAlive}

	go func() {
		time.Sleep(10 * time.Millisecond)
		t.Fatalf("timeout")
	}()
	s.coalesceUpdates()

	if len(d.joined) != 1 || d.joined[0] != m1 {
		t.Fatalf("Expected m1 to join")
	}
	if d.left != nil || d.failed != nil || d.partitioned != nil {
		t.Fatalf("unexpected event")
	}
}

func TestSerf_CoalesceUpdates_Quiescent(t *testing.T) {
	d := &MockDelegate{}
	c := &Config{
		MaxCoalesceTime:  time.Second,
		MinQuiescentTime: time.Millisecond,
		Delegate:         d,
	}
	s := &Serf{conf: c, changeCh: make(chan statusChange, 32)}

	m1 := &Member{}
	m2 := &Member{}

	// M1, none -> alive
	s.changeCh <- statusChange{m1, StatusNone, StatusAlive}
	s.changeCh <- statusChange{m1, StatusAlive, StatusFailed}
	s.changeCh <- statusChange{m1, StatusFailed, StatusPartitioned}
	s.changeCh <- statusChange{m1, StatusPartitioned, StatusAlive}

	// m2 alive -> alive
	s.changeCh <- statusChange{m2, StatusAlive, StatusFailed}
	s.changeCh <- statusChange{m2, StatusFailed, StatusAlive}

	go func() {
		time.Sleep(2 * time.Millisecond)
		t.Fatalf("timeout")
	}()
	s.coalesceUpdates()

	if len(d.joined) != 1 || d.joined[0] != m1 {
		t.Fatalf("Expected m1 to join")
	}
	if d.left != nil || d.failed != nil || d.partitioned != nil {
		t.Fatalf("unexpected event")
	}
}

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

func TestSerf_InvokeDelegate(t *testing.T) {
	d := &MockDelegate{}
	c := &Config{Delegate: d}
	s := &Serf{conf: c}

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

	s.invokeDelegate(init, end)

	if len(d.joined) != 1 || d.joined[0] != m1 {
		t.Fatalf("m1 should have joined!")
	}
	if len(d.left) != 1 || d.left[0] != m4 {
		t.Fatalf("m4 should have left!")
	}
	if len(d.failed) != 1 || d.failed[0] != m2 {
		t.Fatalf("m2 should have failed!")
	}
	if len(d.partitioned) != 1 || d.partitioned[0] != m3 {
		t.Fatalf("m3 should have partitioned!")
	}
}

func TestSerf_InvokeDelegate_NoDelegate(t *testing.T) {
	c := &Config{}
	s := &Serf{conf: c}

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

	s.invokeDelegate(init, end)
}

func TestSerf_NodeJoin_NewNode(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	n := memberlist.Node{Name: "test", Addr: []byte{127, 0, 0, 1}, Meta: []byte("foo")}
	s.nodeJoin(&n)

	mem := s.members["test"]
	if mem.Name != "test" || !reflect.DeepEqual([]byte(mem.Addr), []byte(n.Addr)) || mem.Role != "foo" || mem.Status != StatusAlive {
		t.Fatalf("bad member: %v", *mem)
	}

	ch := <-s.changeCh
	if ch.member != mem || ch.oldStatus != StatusNone || ch.newStatus != StatusAlive {
		t.Fatalf("bad status change %v", ch)
	}
}

func TestSerf_NodeJoin_Existing(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	md := &MockDetector{}
	s.detector = md

	mem := &Member{
		Name:   "test",
		Addr:   []byte{127, 0, 0, 1},
		Role:   "foo",
		Status: StatusFailed,
	}
	s.members["test"] = mem
	s.failedMembers = append(s.failedMembers, &oldMember{mem, time.Now()})

	n := memberlist.Node{Name: "test", Addr: []byte{127, 0, 0, 1}, Meta: []byte("foo")}
	s.nodeJoin(&n)

	if mem.Name != "test" || !reflect.DeepEqual([]byte(mem.Addr), []byte(n.Addr)) || mem.Role != "foo" || mem.Status != StatusAlive {
		t.Fatalf("bad member: %v", *mem)
	}

	ch := <-s.changeCh
	if ch.member != mem || ch.oldStatus != StatusFailed || ch.newStatus != StatusAlive {
		t.Fatalf("bad status change %v", ch)
	}

	// Should unsuspect
	if len(md.unsuspect) != 1 || md.unsuspect[0] != mem {
		t.Fatalf("should unsuspect")
	}

	if len(s.failedMembers) != 0 {
		t.Fatalf("should no longer be a failed member")
	}
}

func TestSerf_NodeLeave_Failed(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	md := &MockDetector{}
	s.detector = md

	s.members["test"] = &Member{
		Name:   "test",
		Addr:   []byte{127, 0, 0, 1},
		Role:   "foo",
		Status: StatusAlive,
	}

	n := memberlist.Node{Name: "test", Addr: []byte{127, 0, 0, 1}, Meta: []byte("foo")}
	s.nodeLeave(&n)

	mem := s.members["test"]
	if mem.Name != "test" || !reflect.DeepEqual([]byte(mem.Addr), []byte(n.Addr)) || mem.Role != "foo" || mem.Status != StatusFailed {
		t.Fatalf("bad member: %v", *mem)
	}

	ch := <-s.changeCh
	if ch.member != mem || ch.oldStatus != StatusAlive || ch.newStatus != StatusFailed {
		t.Fatalf("bad status change %v", ch)
	}

	// Should suspect
	if len(md.suspect) != 1 || md.suspect[0] != mem {
		t.Fatalf("should suspect")
	}

	if len(s.failedMembers) != 1 || s.failedMembers[0].member != mem {
		t.Fatalf("should be in the failed members")
	}
}

func TestSerf_NodeLeave_Graceful(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	md := &MockDetector{}
	s.detector = md

	s.members["test"] = &Member{
		Name:   "test",
		Addr:   []byte{127, 0, 0, 1},
		Role:   "foo",
		Status: StatusLeaving,
	}

	n := memberlist.Node{Name: "test", Addr: []byte{127, 0, 0, 1}, Meta: []byte("foo")}
	s.nodeLeave(&n)

	mem := s.members["test"]
	if mem.Name != "test" || !reflect.DeepEqual([]byte(mem.Addr), []byte(n.Addr)) || mem.Role != "foo" || mem.Status != StatusLeft {
		t.Fatalf("bad member: %v", *mem)
	}

	ch := <-s.changeCh
	if ch.member != mem || ch.oldStatus != StatusLeaving || ch.newStatus != StatusLeft {
		t.Fatalf("bad status change %v", ch)
	}

	// Should not suspect
	if len(md.suspect) != 0 {
		t.Fatalf("should not suspect")
	}

	if len(s.leftMembers) != 1 || s.leftMembers[0].member != mem {
		t.Fatalf("should be in the left members")
	}
}

func TestSerf_NodeLeave_Unknown(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	n := memberlist.Node{Name: "test", Addr: []byte{127, 0, 0, 1}, Meta: []byte("foo")}
	s.nodeLeave(&n)
}

func TestSerf_IntendLeave_Alive(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	s.members["test"] = &Member{
		Name:   "test",
		Addr:   []byte{127, 0, 0, 1},
		Role:   "foo",
		Status: StatusAlive,
	}

	l := leave{Node: "test"}
	if !s.intendLeave(&l) {
		t.Fatalf("expected rebroadcast")
	}

	mem := s.members["test"]
	if mem.Status != StatusLeaving {
		t.Fatalf("bad member: %v", *mem)
	}

	if s.intendLeave(&l) {
		t.Fatalf("expected no rebroadcast")
	}
}

func TestSerf_IntendLeave_Unknown(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

	l := leave{Node: "test"}
	if s.intendLeave(&l) {
		t.Fatalf("unexpected rebroadcast")
	}
}

func TestSerf_IntendLeave_Timeout(t *testing.T) {
	c := &Config{LeaveTimeout: time.Millisecond}
	s := newSerf(c)

	s.members["test"] = &Member{
		Name:   "test",
		Addr:   []byte{127, 0, 0, 1},
		Role:   "foo",
		Status: StatusAlive,
	}

	l := leave{Node: "test"}
	if !s.intendLeave(&l) {
		t.Fatalf("expected rebroadcast")
	}

	mem := s.members["test"]
	if mem.Status != StatusLeaving {
		t.Fatalf("bad member: %v", *mem)
	}

	// Wait for the timeout
	time.Sleep(3 * time.Millisecond)

	// Should be reset
	if mem.Status != StatusAlive {
		t.Fatalf("bad member: %v", *mem)
	}
}
