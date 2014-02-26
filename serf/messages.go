package serf

import (
	"bytes"
	"github.com/ugorji/go/codec"
)

// messageType are the types of gossip messages Serf will send along
// memberlist.
type messageType uint8

const (
	messageLeaveType messageType = iota
	messageJoinType
	messagePushPullType
	messageUserEventType
	messageQueryType
	messageQueryResponseType
)

// filterType is used with a queryFilter to specify the type of
// filter we are sending
type filterType uint8

const (
	filterNodeType filterType = iota
	filterTagType
)

// messageJoin is the message broadcasted after we join to
// associated the node with a lamport clock
type messageJoin struct {
	LTime LamportTime
	Node  string
}

// messageLeave is the message broadcasted to signal the intentional to
// leave.
type messageLeave struct {
	LTime LamportTime
	Node  string
}

// messagePushPullType is used when doing a state exchange. This
// is a relatively large message, but is sent infrequently
type messagePushPull struct {
	LTime        LamportTime            // Current node lamport time
	StatusLTimes map[string]LamportTime // Maps the node to its status time
	LeftMembers  []string               // List of left nodes
	EventLTime   LamportTime            // Lamport time for event clock
	Events       []*userEvents          // Recent events
}

// messageUserEvent is used for user-generated events
type messageUserEvent struct {
	LTime   LamportTime
	Name    string
	Payload []byte
	CC      bool // "Can Coalesce". Zero value is compatible with Serf 0.1
}

// messageQuery is used for query events
type messageQuery struct {
	LTime   LamportTime    // Event lamport time
	ID      uint32         // Query ID, randomly generated
	Addr    []byte         // Source address, used for a direct reply
	Port    uint16         // Source port, used for a direct reply
	Filters []*queryFilter // Potential query filters
	Ack     bool           // True if requesting an ack
	Name    string         // Query name
	Payload []byte         // Query payload
}

// queryFilter is set with a messageUserQuery to filter
// the set of nodes that need to reply
type queryFilter struct {
	Type filterType // Specify the type of filter
	Val  []byte     // Opaque blob, depends on the filter type
}

// messageQueryResponse is used to respond to a query
type messageQueryResponse struct {
	LTime   LamportTime // Event lamport time
	ID      uint32      // Query ID
	From    string      // Node name
	Ack     bool        // Is this an Ack, or reply
	Payload []byte      // Optional response payload
}

func decodeMessage(buf []byte, out interface{}) error {
	var handle codec.MsgpackHandle
	return codec.NewDecoder(bytes.NewBuffer(buf), &handle).Decode(out)
}

func encodeMessage(t messageType, msg interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(uint8(t))

	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(buf, &handle)
	err := encoder.Encode(msg)
	return buf.Bytes(), err
}
