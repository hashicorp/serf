package serf

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message. It's length is limited to
// the given byte size.
func (s *Serf) NodeMeta(limit int) []byte {
	return nil
}

// NotifyMsg is called when a user-data message is received.
// This should not block
func (s *Serf) NotifyMsg([]byte) {
}

// GetBroadcasts is called when user data messages can be broadcast.
// It can return a list of buffers to send. Each buffer should assume an
// overhead as provided with a limit on the total byte size allowed.
func (s *Serf) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState is used for a TCP Push/Pull. This is sent to
// the remote side as well as membership information
func (s *Serf) LocalState() []byte {
	return nil
}

// MergeRemoteState is invoked after a TCP Push/Pull. This is the
// state received from the remote side.
func (s *Serf) MergeRemoteState([]byte) {
}
