package serf

import (
	"github.com/hashicorp/memberlist"
)

const (
	leaveMsg = iota
)

// leave message is broadcast to signal intention to leave
type leave struct {
	Node string
}

type serfBroadcast struct {
	msg    []byte
	notify chan struct{}
}

func (b *serfBroadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *serfBroadcast) Message() []byte {
	return b.msg
}

func (b *serfBroadcast) Finished() {
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// encodeBroadcastNotify encodes a message and enqueues it for broadcast and notifies
// the given channel when transmission is finished
func (s *Serf) encodeBroadcastNotify(msgType int, msg interface{}, notify chan struct{}) error {
	buf, err := encode(msgType, msg)
	if err != nil {
		return err
	}

	// Encode the broadcast
	b := &serfBroadcast{buf.Bytes(), notify}
	s.broadcasts.QueueBroadcast(b)
	return nil
}

// rebroadcast is used to enqueue a message to be rebroadcast
func (s *Serf) rebroadcast(msg []byte) {
	b := &serfBroadcast{msg, nil}
	s.broadcasts.QueueBroadcast(b)
}
