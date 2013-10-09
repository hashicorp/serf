package agent

import (
	"testing"
)

func TestAgentShutdown_multiple(t *testing.T) {
	a := testAgent()
	if err := a.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	for i := 0; i < 5; i++ {
		if err := a.Shutdown(); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
}

func TestAgentShutdown_noStart(t *testing.T) {
	a := testAgent()
	if err := a.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}
}
