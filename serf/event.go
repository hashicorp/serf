package serf

// EventType are all the types of events that may occur and be sent
// along the Serf channel.
type EventType int

const (
	EventMemberJoin EventType = iota
	EventMemberLeave
	EventMemberFailed
)

// Event is the struct sent along the event channel configured for
// Serf. Because Serf coalesces events, an event may contain multiple
// members.
type Event struct {
	Type    EventType
	Members []Member
}
