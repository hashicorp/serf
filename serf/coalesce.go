// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"time"
)

// coalescer is a simple interface that must be implemented to be
// used inside of a coalesceLoop
type coalescer interface {
	// Can the coalescer handle this event, if not it is
	// directly passed through to the destination channel
	Handle(Event) bool

	// Invoked to coalesce the given event
	Coalesce(Event)

	// Invoked to flush the coalesced events
	Flush(outChan chan<- Event)
}

// coalescedEventCh returns an event channel where the events are coalesced
// using the given coalescer.
func coalescedEventCh(outCh chan<- Event, shutdownCh <-chan struct{},
	cPeriod time.Duration, c coalescer) chan<- Event {
	inCh := make(chan Event, 1024)
	go coalesceLoop(inCh, outCh, shutdownCh, cPeriod, c)
	return inCh
}

// coalesceLoop is a simple long-running routine that manages the high-level
// flow of coalescing based on quiescence and a maximum quantum period.
func coalesceLoop(inCh <-chan Event, outCh chan<- Event, shutdownCh <-chan struct{},
	coalescePeriod time.Duration, c coalescer) {
	for {
		select {
		case e := <-inCh:
			// Ignore any non handled events
			if !c.Handle(e) {
				outCh <- e
				continue
			}

			// Coalesce the event
			c.Coalesce(e)

		case <-time.After(coalescePeriod):
			c.Flush(outCh)
		case <-shutdownCh:
			return
		}
	}
}
