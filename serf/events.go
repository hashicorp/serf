package serf

import (
	"github.com/hashicorp/memberlist"
)

// nodeJoin is fired when memberlist detects a node join
func (s *Serf) nodeJoin(n *memberlist.Node) {
}

// nodeLeave is fired when memberlist detects a node join
func (s *Serf) nodeLeave(n *memberlist.Node) {
}
