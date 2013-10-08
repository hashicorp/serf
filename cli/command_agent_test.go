package cli

import (
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

	args := []string{"-bind", getBindAddr().String()}
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
		"-bind", getBindAddr().String(),
		"-rpc-addr", rpcAddr,
	}

	go func() {
		c.Run(args, new(MockUi))
		close(doneCh)
	}()

	yield()
}
