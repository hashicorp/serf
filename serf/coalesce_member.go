// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import "reflect"

// event happens to a node
type nodeEvent struct {
	Type   EventType
	Member *Member
}

func (n *nodeEvent) Equal(m *nodeEvent) bool {
	if m == nil {
		return false
	}
	if n.Type != m.Type {
		return false
	}
	if n.Type != EventMemberUpdate {
		return true
	}
	return reflect.DeepEqual(n.Member, m.Member)
}

type memberEventCoalescer struct {
	lastEvents map[string]*nodeEvent // the last event happens to a node
	newEvents  map[string]*nodeEvent // most recent event for a node.
}

func (c *memberEventCoalescer) Handle(e Event) bool {
	switch e.EventType() {
	case EventMemberJoin:
		return true
	case EventMemberLeave:
		return true
	case EventMemberFailed:
		return true
	case EventMemberUpdate:
		return true
	case EventMemberReap:
		return true
	default:
		return false
	}
}

func (c *memberEventCoalescer) Coalesce(raw Event) {
	e := raw.(MemberEvent)
	for _, m := range e.Members {
		c.newEvents[m.Name] = &nodeEvent{ // overwrite the old events
			Type:   e.Type,
			Member: &m,
		}
	}
}
func (c *memberEventCoalescer) Flush(outCh chan<- Event) {
	// Coalesce the various events we got into a single set of events.
	events := make(map[EventType]*MemberEvent)
	for name, e := range c.newEvents {
		if e.Equal(c.lastEvents[name]) {
			continue
		}

		// Update our last event
		c.lastEvents[name] = e

		// Add it to our event
		event, ok := events[e.Type]
		if !ok {
			event = &MemberEvent{Type: e.Type}
			events[e.Type] = event
		}
		event.Members = append(event.Members, *e.Member)
	}

	// Send out those events
	for _, event := range events {
		outCh <- *event
	}
}
