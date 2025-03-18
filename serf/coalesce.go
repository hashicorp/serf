// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"time"
)

// Coalescer is a simple interface that must be implemented to be
// used inside of a coalesceLoop
type Coalescer interface {
	// Can the Coalescer handle this event, if not it is
	// directly passed through to the destination channel
	Handle(Event) bool

	// Invoked to coalesce the given event
	Coalesce(Event)

	// Invoked to flush the coalesced events
	Flush(outChan chan<- Event)
}

// coalescedEventCh returns an event channel where the events are coalesced
// using the given Coalescer.
func coalescedEventCh(outCh chan<- Event, shutdownCh <-chan struct{},
	cPeriod time.Duration, qPeriod time.Duration, c Coalescer) chan<- Event {
	inCh := make(chan Event, 1024)
	go coalesceLoop(inCh, outCh, shutdownCh, cPeriod, qPeriod, c)
	return inCh
}

// coalesceLoop is a simple long-running routine that manages the high-level
// flow of coalescing based on quiescence and a maximum quantum period.
func coalesceLoop(inCh <-chan Event, outCh chan<- Event, shutdownCh <-chan struct{},
	coalescePeriod time.Duration, quiescentPeriod time.Duration, c Coalescer) {
	var quiescent <-chan time.Time
	var quantum <-chan time.Time
	shutdown := false

INGEST:
	// Reset the timers
	quantum = nil
	quiescent = nil

	for {
		select {
		case e := <-inCh:
			// Ignore any non handled events
			if !c.Handle(e) {
				outCh <- e
				continue
			}

			// Start a new quantum if we need to
			// and restart the quiescent timer
			if quantum == nil {
				quantum = time.After(coalescePeriod)
			}
			quiescent = time.After(quiescentPeriod)

			// Coalesce the event
			c.Coalesce(e)

		case <-quantum:
			goto FLUSH
		case <-quiescent:
			goto FLUSH
		case <-shutdownCh:
			shutdown = true
			goto FLUSH
		}
	}

FLUSH:
	// Flush the coalesced events
	c.Flush(outCh)

	// Restart ingestion if we are not done
	if !shutdown {
		goto INGEST
	}
}
