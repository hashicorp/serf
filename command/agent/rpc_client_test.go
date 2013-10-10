package agent

import (
	"github.com/hashicorp/serf/testutil"
	"net"
	"net/rpc"
	"testing"
)

// testRPCClient returns an RPCClient connected to an RPC server that
// serves only this connection.
func testRPCClient(t *testing.T) (*RPCClient, *Agent) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	agent := testAgent()
	server := rpc.NewServer()
	if err := registerEndpoint(server, agent); err != nil {
		l.Close()
		t.Fatalf("err: %s", err)
	}

	go func() {
		conn, err := l.Accept()
		l.Close()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		defer conn.Close()
		server.ServeConn(conn)
	}()

	rpcClient, err := rpc.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return &RPCClient{Client: rpcClient}, agent
}

func TestRPCClientJoin(t *testing.T) {
	client, a1 := testRPCClient(t)
	a2 := testAgent()
	defer client.Close()
	defer a1.Shutdown()
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	n, err := client.Join([]string{a2.SerfConfig.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("n != 1: %d", n)
	}
}

func TestRPCClientMembers(t *testing.T) {
	client, a1 := testRPCClient(t)
	a2 := testAgent()
	defer client.Close()
	defer a1.Shutdown()
	defer a2.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	mem, err := client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 1 {
		t.Fatalf("bad: %#v", mem)
	}

	_, err = client.Join([]string{a2.SerfConfig.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	mem, err = client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 2 {
		t.Fatalf("bad: %#v", mem)
	}
}
