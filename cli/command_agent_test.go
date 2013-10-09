package cli

import (
	serfrpc "github.com/hashicorp/serf/rpc"
	"github.com/hashicorp/serf/testutil"
	"log"
	"net/rpc"
	"testing"
	"time"
)

func TestAgentCommand_implements(t *testing.T) {
	var raw interface{}
	raw = &AgentCommand{}
	if _, ok := raw.(Command); !ok {
		t.Fatal("should be a Command")
	}
}

func TestAgentCommandRun(t *testing.T) {
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &AgentCommand{
		ShutdownCh: shutdownCh,
	}

	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-rpc-addr", getRPCAddr(),
	}
	ui := new(MockUi)

	resultCh := make(chan int)
	go func() {
		resultCh <- c.Run(args, ui)
	}()

	yield()

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

func TestAgentCommandRun_rpc(t *testing.T) {
	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})
	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	c := &AgentCommand{
		ShutdownCh: shutdownCh,
	}

	rpcAddr := getRPCAddr()
	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-rpc-addr", rpcAddr,
	}

	go func() {
		code := c.Run(args, new(MockUi))
		if code != 0 {
			log.Printf("bad: %d", code)
		}

		close(doneCh)
	}()

	yield()

	rpcClient, err := rpc.Dial("tcp", rpcAddr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer rpcClient.Close()

	client := serfrpc.NewClient(rpcClient)
	members, err := client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %#v", members)
	}
}
