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
	c := &AgentCommand{
		ShutdownCh: shutdownCh,
	}

	resultCh := make(chan int)
	ui := new(MockUi)
	go func() {
		resultCh <- c.Run(nil, ui)
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
