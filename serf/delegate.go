package serf

import (
	"fmt"
)

// delegate is the memberlist.Delegate implementation that Serf uses.
type delegate struct {
	serf *Serf
}

func (d *delegate) NodeMeta(limit int) []byte {
	roleBytes := []byte(d.serf.config.Role)
	if len(roleBytes) > limit {
		panic(fmt.Errorf("role '%s' exceeds length limit of %d bytes", d.serf.config.Role, limit))
	}

	return roleBytes
}

func (d *delegate) NotifyMsg(buf []byte) {
	// If we didn't actually receive any data, then ignore it.
	if len(buf) == 0 {
		return
	}

	rebroadcast := false
	t := messageType(buf[0])
	switch t {
	case messageLeaveType:
		var leave messageLeave
		if err := decodeMessage(buf[1:], &leave); err != nil {
			d.serf.logger.Printf("[ERR] Error decoding leave message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf-delegate: messageLeaveType: %s", leave.Node)
		rebroadcast = d.serf.handleNodeLeaveIntent(&leave)

	case messageRemoveFailedType:
		var remove messageRemoveFailed
		if err := decodeMessage(buf[1:], &remove); err != nil {
			d.serf.logger.Printf("[ERR] Error decoding remove message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf-delegate: messageRemoveFailedType: %s", remove.Node)
		rebroadcast = d.serf.handleNodeForceRemove(&remove)

	case messageJoinType:
		var join messageJoin
		if err := decodeMessage(buf[1:], &join); err != nil {
			d.serf.logger.Printf("[ERR] Error decoding join message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf-delegate: messageJoinType: %s", join.Node)
		rebroadcast = d.serf.handleNodeJoinIntent(&join)

	default:
		d.serf.logger.Printf("[WARN] Received message of unknown type: %d", t)
	}

	if rebroadcast {
		d.serf.broadcasts.QueueBroadcast(&broadcast{
			msg:    buf,
			notify: nil,
		})
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	msgs := d.serf.broadcasts.GetBroadcasts(overhead, limit)

	if msgs != nil {
		numq := d.serf.broadcasts.NumQueued()
		limit := d.serf.config.QueueDepthWarning
		if numq >= limit {
			d.serf.logger.Printf("[WARN] Broadcast queue depth: %d", numq)
		}
	}

	return msgs
}

func (d *delegate) LocalState() []byte {
	return nil
}

func (d *delegate) MergeRemoteState([]byte) {
}
