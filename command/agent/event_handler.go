package agent

import (
	"github.com/hashicorp/serf/serf"
	"log"
)

// EventHandler is a handler that does things when events happen.
type EventHandler interface {
	HandleEvent(*log.Logger, serf.Event) error
}

// ScriptEventHandler invokes scripts for the events that it receives.
type ScriptEventHandler struct {
	Scripts []EventScript
}

func (h *ScriptEventHandler) HandleEvent(logger *log.Logger, e serf.Event) error {
	for _, script := range h.Scripts {
		if !script.Invoke(e) {
			continue
		}

		err := invokeEventScript(logger, script.Script, e)
		if err != nil {
			logger.Printf("[ERR] Error invoking script '%s': %s",
				script.Script, err)
		}
	}

	return nil
}
