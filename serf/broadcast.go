package serf

import (
	"github.com/hashicorp/memberlist"
)

// broadcast is an implementation of memberlist.Broadcast and is used
// to manage broadcasts across the memberlist channel that are related
// only to Serf.
type broadcast struct {
	key    string
	msg    []byte
	notify chan<- struct{}
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	if b2, ok := other.(*broadcast); !ok {
		return false
	} else {
		return b.key == b2.key
	}
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}
