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
	EventMemberUpdate
	EventUser
	EventQuery
)

func (t EventType) String() string {
	switch t {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	case EventMemberUpdate:
		return "member-update"
	case EventUser:
		return "user"
	case EventQuery:
		return "query"
	default:
		panic(fmt.Sprintf("unknown event type: %d", t))
	}
}

// Event is a generic interface for exposing Serf events
// Clients will usually need to use a type switches to get
// to a more useful type
type Event interface {
	EventType() EventType
	String() string
}

// MemberEvent is the struct used for member related events
// Because Serf coalesces events, an event may contain multiple members.
type MemberEvent struct {
	Type    EventType
	Members []Member
}

func (m MemberEvent) EventType() EventType {
	return m.Type
}

func (m MemberEvent) String() string {
	switch m.Type {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	case EventMemberUpdate:
		return "member-update"
	default:
		panic(fmt.Sprintf("unknown event type: %d", m.Type))
	}
}

// UserEvent is the struct used for events that are triggered
// by the user and are not related to members
type UserEvent struct {
	LTime    LamportTime
	Name     string
	Payload  []byte
	Coalesce bool
}

func (u UserEvent) EventType() EventType {
	return EventUser
}

func (u UserEvent) String() string {
	return fmt.Sprintf("user-event: %s", u.Name)
}

// Query is the struct used EventQuery type events
type Query struct {
	LTime   LamportTime
	Name    string
	Payload []byte

	id   uint32 // ID is not exported, since it may change
	addr []byte // Address to respond to
	port uint16 // Port to respond to
}

func (q Query) EventType() EventType {
	return EventQuery
}

func (q Query) String() string {
	return fmt.Sprintf("query: %s", q.Name)
}

// Respond is used to send a response to the user query
func (q Query) Respond([]byte) {
	// TODO: Send response
}
