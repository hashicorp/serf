package agent

import (
	"github.com/hashicorp/serf/cli"
	"github.com/hashicorp/serf/testutil"
	"log"
	"net/rpc"
	"testing"
	"time"
)

func TestCommand_implements(t *testing.T) {
	var raw interface{}
	raw = &Command{}
	if _, ok := raw.(cli.Command); !ok {
		t.Fatal("should be a Command")
	}
}

func TestCommandRun(t *testing.T) {
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &Command{
		ShutdownCh: shutdownCh,
	}

	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-rpc-addr", getRPCAddr(),
	}
	ui := new(cli.MockUi)

	resultCh := make(chan int)
	go func() {
		resultCh <- c.Run(args, ui)
	}()

	testutil.Yield()

	// Verify it runs "forever"
	select {
	case <-resultCh:
		t.Fatalf("ended too soon, err: %s", ui.ErrorWriter.String())
	case <-time.After(50 * time.Millisecond):
	}

	// Send a shutdown request
	shutdownCh <- struct{}{}

	select {
	case code := <-resultCh:
		if code != 0 {
			t.Fatalf("bad code: %d", code)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}

func TestCommandRun_rpc(t *testing.T) {
	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})
	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	c := &Command{
		ShutdownCh: shutdownCh,
	}

	rpcAddr := getRPCAddr()
	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-rpc-addr", rpcAddr,
	}

	go func() {
		code := c.Run(args, new(cli.MockUi))
		if code != 0 {
			log.Printf("bad: %d", code)
		}

		close(doneCh)
	}()

	testutil.Yield()

	rpcConn, err := rpc.Dial("tcp", rpcAddr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer rpcConn.Close()

	client := &RPCClient{Client: rpcConn}
	members, err := client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %#v", members)
	}
}
