package rpc

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"math/rand"
	"net"
	"net/rpc"
	"testing"
)

// testClient creates a new server for the given serf and returns
// a client that will work for a single RPC call before ending.
func testClient(t *testing.T, serf *serf.Serf) *Client {
	// Make the listener that we just close after a single connection
	var l net.Listener
	for i := 0; i < 500; i++ {
		var err error
		l, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", rand.Int31n(25000)+1024))
		if err == nil {
			break
		}

		l = nil
	}

	if l == nil {
		t.Fatalf("no listener could be made")
	}

	server, err := NewServer(serf, l)
	if err != nil {
		l.Close()
		t.Fatalf("err: %s", err)
	}

	// Serve a single connection
	go func() {
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		server.ServeConn(conn)
	}()

	// Create the client
	client, err := rpc.Dial("tcp", l.Addr().String())
	if err != nil {
		l.Close()
		t.Fatalf("err: %s", err)
	}

	return NewClient(client)
}

func TestClientJoin(t *testing.T) {
	s1, _ := testSerf(t)
	s2, s2Addr := testSerf(t)
	defer s1.Shutdown()
	defer s2.Shutdown()

	c := testClient(t, s1)
	n, err := c.Join([]string{s2Addr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("bad: %d", n)
	}

	if len(s1.Members()) != 2 {
		t.Fatalf("bad: %#v", s1.Members())
	}
}

func TestClientMembers(t *testing.T) {
	s1, _ := testSerf(t)
	defer s1.Shutdown()

	c := testClient(t, s1)
	m, err := c.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(m) != 1 {
		t.Fatalf("bad: %#v", m)
	}
}
