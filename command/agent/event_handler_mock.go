package agent

import (
	"github.com/hashicorp/serf/serf"
	"log"
	"sync"
)

// MockEventHandler is an EventHandler implementation that can be used
// for tests.
type MockEventHandler struct {
	Events []serf.Event

	sync.Mutex
}

func (h *MockEventHandler) HandleEvent(l *log.Logger, e serf.Event) error {
	h.Lock()
	defer h.Unlock()

	h.Events = append(h.Events, e)
	return nil
}
