package serf

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestInternalQueryName(t *testing.T) {
	name := internalQueryName(conflictQuery)
	if name != "_serf_conflict" {
		t.Fatalf("bad: %v", name)
	}
}

func TestSerfQueries_Passthrough(t *testing.T) {
	serf := &Serf{}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	outCh := make(chan Event, 4)
	shutdown := make(chan struct{})
	defer close(shutdown)
	eventCh, err := newSerfQueries(serf, logger, outCh, shutdown)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Push a user event
	eventCh <- UserEvent{LTime: 42, Name: "foo"}

	// Push a query
	eventCh <- &Query{LTime: 42, Name: "foo"}

	// Push a query
	eventCh <- MemberEvent{Type: EventMemberJoin}

	// Should get passed through
	for i := 0; i < 3; i++ {
		select {
		case <-outCh:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("time out")
		}
	}
}

func TestSerfQueries_Ping(t *testing.T) {
	serf := &Serf{}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	outCh := make(chan Event, 4)
	shutdown := make(chan struct{})
	defer close(shutdown)
	eventCh, err := newSerfQueries(serf, logger, outCh, shutdown)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Send a ping
	eventCh <- &Query{LTime: 42, Name: "_serf_ping"}

	// Should not get passed through
	select {
	case <-outCh:
		t.Fatalf("Should not passthrough query!")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSerfQueries_Conflict_SameName(t *testing.T) {
	serf := &Serf{config: &Config{NodeName: "foo"}}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	outCh := make(chan Event, 4)
	shutdown := make(chan struct{})
	defer close(shutdown)
	eventCh, err := newSerfQueries(serf, logger, outCh, shutdown)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query for our own name
	eventCh <- &Query{Name: "_serf_conflict", Payload: []byte("foo")}

	// Should not passthrough OR respond
	select {
	case <-outCh:
		t.Fatalf("Should not passthrough query!")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSerfQueries_keyListResponseWithCorrectSize(t *testing.T) {
	s := serfQueries{logger: log.New(os.Stderr, "", log.LstdFlags)}
	q := Query{id: 1136243741, serf: &Serf{config: &Config{NodeName: "foo.dc1", QueryResponseSizeLimit: 160}}}
	cases := []struct {
		resp     nodeKeyResponse
		expected int
	}{
		{expected: 0, resp: nodeKeyResponse{}},
		{expected: 1, resp: nodeKeyResponse{Keys: []string{"LeJcrRIZsZ9tPYJZW7Xllg=="}}},
		// has 5 keys which makes the response bigger than 160 bytes.
		{expected: 3, resp: nodeKeyResponse{Keys: []string{"LeJcrRIZsZ9tPYJZW7Xllg==", "LeJcrRIZsZ9tPYJZW7Xllg==", "LeJcrRIZsZ9tPYJZW7Xllg==", "LeJcrRIZsZ9tPYJZW7Xllg==", "LeJcrRIZsZ9tPYJZW7Xllg=="}}},
	}
	for _, c := range cases {
		r := c.resp
		_, _, err := s.keyListResponseWithCorrectSize(&q, &r)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Keys) != c.expected {
			t.Fatalf("Expected %d vs %d", c.expected, len(r.Keys))
		}
	}
}
