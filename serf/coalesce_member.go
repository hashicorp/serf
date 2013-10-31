package serf

import (
	"time"
)

type coalesceEvent struct {
	Type   EventType
	Member *Member
}

type memberEventCoalescer struct {
	lastEvents   map[string]EventType
	latestEvents map[string]coalesceEvent
}

// coalescedMemberEventCh returns an event channel where member events are coalesced
// over a period of time. This helps lower the number of events that are
// fired in the case where many nodes share similar events at one time.
// Examples where this is possible are if many new nodes are brought online
// at one time, events will be coalesced together into one event.
func coalescedMemberEventCh(outCh chan<- Event, shutdownCh <-chan struct{},
	coalescePeriod time.Duration, quiescentPeriod time.Duration) chan<- Event {
	inCh := make(chan Event, 1024)
	c := &memberEventCoalescer{
		lastEvents:   make(map[string]EventType),
		latestEvents: make(map[string]coalesceEvent),
	}

	go coalesceLoop(inCh, outCh, shutdownCh, coalescePeriod, quiescentPeriod, c)
	return inCh
}

func (c *memberEventCoalescer) Handle(e Event) bool {
	switch e.EventType() {
	case EventMemberJoin:
		return true
	case EventMemberLeave:
		return true
	case EventMemberFailed:
		return true
	default:
		return false
	}
}

func (c *memberEventCoalescer) Coalesce(raw Event) {
	e := raw.(MemberEvent)
	for _, m := range e.Members {
		c.latestEvents[m.Name] = coalesceEvent{
			Type:   e.Type,
			Member: &m,
		}
	}
}

func (c *memberEventCoalescer) Flush(outCh chan<- Event) {
	// Coalesce the various events we got into a single set of events.
	events := make(map[EventType]*MemberEvent)
	for name, cevent := range c.latestEvents {
		previous, ok := c.lastEvents[name]

		// If we sent the same event before, then ignore
		if ok && previous == cevent.Type {
			continue
		}

		// Update our last event
		c.lastEvents[name] = cevent.Type

		// Add it to our event
		newEvent, ok := events[cevent.Type]
		if !ok {
			newEvent = &MemberEvent{Type: cevent.Type}
			events[cevent.Type] = newEvent
		}
		newEvent.Members = append(newEvent.Members, *cevent.Member)
	}

	// Send out those events
	for _, event := range events {
		outCh <- *event
	}
}
