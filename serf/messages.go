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
