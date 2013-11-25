package agent

import (
	"github.com/hashicorp/serf/serf"
	"sync"
)

// MockEventHandler is an EventHandler implementation that can be used
// for tests.
type MockEventHandler struct {
	Events []serf.Event
	sync.Mutex
}

func (h *MockEventHandler) HandleEvent(e serf.Event) {
	h.Lock()
	defer h.Unlock()
	h.Events = append(h.Events, e)
}
