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
	rebroadcastQueue := d.serf.broadcasts
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

	case messageJoinType:
		var join messageJoin
		if err := decodeMessage(buf[1:], &join); err != nil {
			d.serf.logger.Printf("[ERR] Error decoding join message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf-delegate: messageJoinType: %s", join.Node)
		rebroadcast = d.serf.handleNodeJoinIntent(&join)

	case messageUserEventType:
		var event messageUserEvent
		if err := decodeMessage(buf[1:], &event); err != nil {
			d.serf.logger.Printf("[ERR] Error decoding user event message: %s", err)
			break
		}

		d.serf.logger.Printf("[DEBUG] serf-delegate: messageUserEventType: %s", event.Name)
		rebroadcast = d.serf.handleUserEvent(&event)
		rebroadcastQueue = d.serf.eventBroadcasts

	default:
		d.serf.logger.Printf("[WARN] Received message of unknown type: %d", t)
	}

	if rebroadcast {
		// Copy the buffer since it we cannot rely on the slice not changing
		newBuf := make([]byte, len(buf))
		copy(newBuf, buf)

		rebroadcastQueue.QueueBroadcast(&broadcast{
			msg:    newBuf,
			notify: nil,
		})
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	msgs := d.serf.broadcasts.GetBroadcasts(overhead, limit)

	// Determine the bytes used already
	bytesUsed := 0
	for _, msg := range msgs {
		bytesUsed += len(msg) + overhead
	}

	// Get any additional event broadcasts
	eventMsgs := d.serf.eventBroadcasts.GetBroadcasts(overhead, limit-bytesUsed)
	if eventMsgs != nil {
		msgs = append(msgs, eventMsgs...)
	}

	return msgs
}

func (d *delegate) LocalState() []byte {
	d.serf.memberLock.RLock()
	defer d.serf.memberLock.RUnlock()
	d.serf.eventLock.RLock()
	defer d.serf.eventLock.RUnlock()

	// Create the message to send
	pp := messagePushPull{
		LTime:        d.serf.clock.Time(),
		StatusLTimes: make(map[string]LamportTime, len(d.serf.members)),
		LeftMembers:  make([]string, 0, len(d.serf.leftMembers)),
		EventLTime:   d.serf.eventClock.Time(),
		Events:       d.serf.eventBuffer,
	}

	// Add all the join LTimes
	for name, member := range d.serf.members {
		pp.StatusLTimes[name] = member.statusLTime
	}

	// Add all the left nodes
	for _, member := range d.serf.leftMembers {
		pp.LeftMembers = append(pp.LeftMembers, member.Name)
	}

	// Encode the push pull state
	buf, err := encodeMessage(messagePushPullType, &pp)
	if err != nil {
		d.serf.logger.Printf("[ERR] serf: Failed to encode local state: %v", err)
		return nil
	}
	return buf
}

func (d *delegate) MergeRemoteState(buf []byte) {
	// Check the message type
	if messageType(buf[0]) != messagePushPullType {
		d.serf.logger.Printf("[ERR] serf: Remote state has bad type prefix: %v", buf[0])
		return
	}

	// Attempt a decode
	pp := messagePushPull{}
	if err := decodeMessage(buf[1:], &pp); err != nil {
		d.serf.logger.Printf("[ERR] serf: Failed to decode remote state: %v", err)
		return
	}

	// Witness the Lamport clocks first.
	// We subtract 1 since no message with that clock has been sent yet
	d.serf.clock.Witness(pp.LTime - 1)
	d.serf.eventClock.Witness(pp.EventLTime - 1)

	// Process the left nodes first to avoid the LTimes from being increment
	// in the wrong order
	leftMap := make(map[string]struct{}, len(pp.LeftMembers))
	leave := messageLeave{}
	for _, name := range pp.LeftMembers {
		leftMap[name] = struct{}{}
		leave.LTime = pp.StatusLTimes[name]
		leave.Node = name
		d.serf.handleNodeLeaveIntent(&leave)
	}

	// Update any other LTimes
	join := messageJoin{}
	for name, statusLTime := range pp.StatusLTimes {
		// Skip the left nodes
		if _, ok := leftMap[name]; ok {
			continue
		}

		// Create an artificial join message
		join.LTime = statusLTime
		join.Node = name
		d.serf.handleNodeJoinIntent(&join)
	}

	// Process all the events
	userEvent := messageUserEvent{}
	for _, events := range pp.Events {
		if events == nil {
			continue
		}
		userEvent.LTime = events.LTime
		for _, e := range events.Events {
			userEvent.Name = e.Name
			userEvent.Payload = e.Payload
			d.serf.handleUserEvent(&userEvent)
		}
	}
}
