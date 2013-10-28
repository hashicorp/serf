package serf

import (
	"time"
)

type latestUserEvents struct {
	LTime  LamportTime
	Events []Event
}

type userEventCoalescer struct {
	// Maps an event name into the latest versions
	events map[string]*latestUserEvents
}

// coalescedUserEventCh returns an event channel where user events are coalesced
// over a period of time. This helps lower the number of events that are
// fired in the case where many nodes fire events at one time. We coalesce by
// selecting the event with the highest lamport time for a given event name.
// If multiple events exist for a given lamport time, all of them will be returned
func coalescedUserEventCh(outCh chan<- Event, shutdownCh <-chan struct{},
	cPeriod time.Duration, qPeriod time.Duration) chan<- Event {
	inCh := make(chan Event, 1024)
	c := newUserEventCoalescer()
	go coalesceLoop(inCh, outCh, shutdownCh, cPeriod, qPeriod, c)
	return inCh
}

func newUserEventCoalescer() *userEventCoalescer {
	return &userEventCoalescer{
		events: make(map[string]*latestUserEvents),
	}
}

func (c *userEventCoalescer) Handle(e Event) bool {
	return e.EventType() == EventUser
}

func (c *userEventCoalescer) Coalesce(e Event) {
	user := e.(UserEvent)
	latest, ok := c.events[user.Name]

	// Create a new entry if there are none, or
	// if this message has the newest LTime
	if !ok || latest.LTime < user.LTime {
		latest = &latestUserEvents{
			LTime:  user.LTime,
			Events: []Event{e},
		}
		c.events[user.Name] = latest
		return
	}

	// If the the same age, save it
	if latest.LTime == user.LTime {
		latest.Events = append(latest.Events, e)
	}
}

func (c *userEventCoalescer) Flush(outChan chan<- Event) {
	for _, latest := range c.events {
		for _, e := range latest.Events {
			outChan <- e
		}
	}
	c.events = make(map[string]*latestUserEvents)
}
