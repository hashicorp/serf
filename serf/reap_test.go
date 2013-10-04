package serf

import (
	"testing"
	"time"
)

func TestSerf_ReapHandler_Shutdown(t *testing.T) {
	c := &Config{}
	s := newSerf(c)
	close(s.shutdownCh)
	go func() {
		time.Sleep(time.Millisecond)
		t.Fatalf("timeout")
	}()
	s.reapHandler()
}

func TestSerf_ReapHandler(t *testing.T) {
	c := &Config{
		ReapInterval:     time.Nanosecond,
		TombstoneTimeout: time.Second * 6,
	}
	s := newSerf(c)

	m := &Member{}
	s.leftMembers = []*oldMember{
		&oldMember{m, time.Now()},
		&oldMember{m, time.Now().Add(-5 * time.Second)},
		&oldMember{m, time.Now().Add(-10 * time.Second)},
	}

	go func() {
		time.Sleep(time.Millisecond)
		close(s.shutdownCh)
	}()

	s.reapHandler()

	if len(s.leftMembers) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestSerf_Reap(t *testing.T) {
	c := &Config{}
	s := newSerf(c)

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
	m := &Member{}
	o := &oldMember{member: m}
	old := []*oldMember{
		&oldMember{},
		o,
		&oldMember{},
	}

	old = removeOldMember(old, m)
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
	if old[1] == o {
		t.Fatalf("should remove old member")
	}
}
