package serf

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

var bindLock sync.Mutex
var (
	bindNum = 10
)

func GetSerf(t *testing.T) *Serf {
	c := DefaultConfig()
	c.GossipBindAddr = "127.0.0.1"

	var s *Serf
	var err error
	for i := 0; i < 100; i++ {
		s, err = Start(c)
		if err == nil {
			return s
		}
		c.GossipPort++
	}
	t.Fatalf("failed to start: %v", err)
	return nil
}

func GetBindAddr() (string, []byte) {
	bindLock.Lock()
	defer bindLock.Unlock()
	addr := bindNum
	bindNum++
	s := fmt.Sprintf("127.0.0.%d", addr)
	b := []byte{127, 0, 0, byte(addr)}
	return s, b
}

func TestSerf_CreateShutdown(t *testing.T) {
	s := GetSerf(t)
	if err := s.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown %v", err)
	}
}

func TestSerf_Join(t *testing.T) {
	s := GetSerf(t)
	defer s.Shutdown()

	c := DefaultConfig()
	addr1, _ := GetBindAddr()
	c.Hostname = addr1
	c.GossipBindAddr = addr1
	c.GossipPort = s.conf.GossipPort
	s2, err := Start(c)
	if err != nil {
		t.Fatal("unexpected err: %s", err)
	}
	defer s2.Shutdown()

	err = s2.Join([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal("unexpected err: %s", err)
	}

	// Yield for a bit to allow `s` to process updates
	time.Sleep(time.Millisecond)

	if len(s2.Members()) != 2 {
		t.Fatalf("expected 2 members")
	}
	if len(s.Members()) != 2 {
		t.Fatalf("expected 2 members")
	}
}

func TestSerf_Leave(t *testing.T) {
	s := GetSerf(t)
	defer s.Shutdown()

	c := DefaultConfig()
	addr1, _ := GetBindAddr()
	c.Hostname = addr1
	c.GossipBindAddr = addr1
	c.GossipPort = s.conf.GossipPort
	s2, err := Start(c)
	if err != nil {
		t.Fatal("unexpected err: %s", err)
	}

	err = s2.Join([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal("unexpected err: %s", err)
	}

	// Yield for a bit to allow `s` to process updates
	time.Sleep(time.Millisecond)

	if len(s2.Members()) != 2 {
		t.Fatalf("expected 2 members")
	}
	if len(s.Members()) != 2 {
		t.Fatalf("expected 2 members")
	}

	// Try to gracefully leave
	s2.Leave()
	s2.Shutdown()

	// s should see the member as "left"
	members := s.Members()
	if len(members) != 2 && members[1].Status != StatusLeft {
		t.Fatalf("expected node to leave")
	}
}
