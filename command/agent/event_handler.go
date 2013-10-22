package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"log"
	"strings"
)

// EventHandler is a handler that does things when events happen.
type EventHandler interface {
	HandleEvent(*log.Logger, serf.Event) error
}

// ScriptEventHandler invokes scripts for the events that it receives.
type ScriptEventHandler struct {
	Self    serf.Member
	Scripts []EventScript
}

func (h *ScriptEventHandler) HandleEvent(logger *log.Logger, e serf.Event) error {
	for _, script := range h.Scripts {
		if !script.Invoke(e) {
			continue
		}

		err := invokeEventScript(logger, script.Script, h.Self, e)
		if err != nil {
			logger.Printf("[ERR] Error invoking script '%s': %s",
				script.Script, err)
		}
	}

	return nil
}

// EventScript is a single event script that will be executed in the
// case of an event, and is configured from the command-line or from
// a configuration file.
type EventScript struct {
	Event     string
	UserEvent string
	Script    string
}

// Invoke tests whether or not this event script should be invoked
// for the given Serf event.
func (s *EventScript) Invoke(e serf.Event) bool {
	if s.Event == "*" {
		return true
	}

	if e.EventType().String() != s.Event {
		return false
	}

	if s.UserEvent != "" {
		userE, ok := e.(serf.UserEvent)
		if !ok {
			return false
		}

		if userE.Name != s.UserEvent {
			return false
		}
	}

	return true
}

func (s *EventScript) String() string {
	return fmt.Sprintf("Event '%s' invoking '%s'", s.Event, s.Script)
}

// Valid checks if this is a valid agent event script.
func (s *EventScript) Valid() bool {
	switch s.Event {
	case "member-join":
	case "member-leave":
	case "member-failed":
	case "user":
	case "*":
	default:
		return false
	}

	return true
}

// ParseEventScript takes a string in the format of "type=script" and
// parses it into an EventScript struct, if it can.
func ParseEventScript(v string) ([]EventScript, error) {
	if strings.Index(v, "=") == -1 {
		v = "*=" + v
	}

	parts := strings.SplitN(v, "=", 2)
	events := strings.Split(parts[0], ",")
	results := make([]EventScript, 0, len(events))
	for _, event := range events {
		var result EventScript
		var userEvent string

		if strings.HasPrefix(event, "user:") {
			userEvent = event[len("user:"):]
			event = "user"
		}

		result.Event = event
		result.UserEvent = userEvent
		result.Script = parts[1]
		results = append(results, result)
	}

	return results, nil
}
