package serf

import (
	"fmt"
)

// EventType are all the types of events that may occur and be sent
// along the Serf channel.
type EventType int

const (
	EventMemberJoin EventType = iota
	EventMemberLeave
	EventMemberFailed
)

func (t EventType) String() string {
	switch t {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	default:
		panic(fmt.Sprintf("unknown event type: %d", t))
	}
}

// Event is the struct sent along the event channel configured for
// Serf. Because Serf coalesces events, an event may contain multiple
// members.
type Event struct {
	Type    EventType
	Members []Member
}
