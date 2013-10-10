package serf

import (
	"time"
)

type coalesceEvent struct {
	Type   EventType
	Member *Member
}

// coalescedEventCh returns an event channel where the events are coalesced
// over a period of time. This helps lower the number of events that are
// fired in the case where many nodes share similar events at one time.
// Examples where this is possible are if many new nodes are brought online
// at one time, events will be coalesced together into one event.
func coalescedEventCh(outCh chan<- Event, shutdownCh <-chan struct{},
	coalescePeriod time.Duration, quiescentPeriod time.Duration) chan<- Event {
	eventCh := make(chan Event, 1024)
	go coalescer(eventCh, outCh, shutdownCh, coalescePeriod, quiescentPeriod)
	return eventCh
}

// coalescer is the function that actually does the coalescence work.
// This function runs in a goroutine, processing events until the
// shutdown channel sends a message.
func coalescer(eventCh <-chan Event, newCh chan<- Event, shutdownCh <-chan struct{},
	coalescePeriod time.Duration, quiescentPeriod time.Duration) {
	var quiescent <-chan time.Time
	var quantum <-chan time.Time

	lastEvents := make(map[string]EventType)
	latestEvents := make(map[string]coalesceEvent)
	shutdown := false

	for {
		coalesce := false

		select {
		case rawEvent := <-eventCh:
			// Ignore any non-member related events
			eventType := rawEvent.EventType()
			if eventType != EventMemberJoin && eventType != EventMemberLeave && eventType != EventMemberFailed {
				newCh <- rawEvent
				continue
			}

			// Cast to a member event
			e := rawEvent.(MemberEvent)

			// Start a new quantum if we need to and update the quiescent
			// timer.
			if quantum == nil {
				quantum = time.After(coalescePeriod)
			}
			quiescent = time.After(quiescentPeriod)

			for _, m := range e.Members {
				latestEvents[m.Name] = coalesceEvent{
					Type:   e.Type,
					Member: &m,
				}
			}
		case <-quantum:
			coalesce = true
		case <-quiescent:
			coalesce = true
		case <-shutdownCh:
			// Make sure we coalesce one last time and then shut down
			coalesce = true
			shutdown = true
		}

		if !coalesce {
			continue
		}

		// Reset the timers
		quantum = nil
		quiescent = nil

		// Coalesce the various events we got into a single set of events.
		events := make(map[EventType]*MemberEvent)
		for name, cevent := range latestEvents {
			previous, ok := lastEvents[name]

			// If we sent the same event before, then ignore
			if ok && previous == cevent.Type {
				continue
			}

			// Update our last event
			lastEvents[name] = cevent.Type

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
			newCh <- *event
		}

		// If we were told to shutdown, then exit now
		if shutdown {
			break
		}
	}
}
