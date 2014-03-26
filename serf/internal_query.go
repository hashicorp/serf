package serf

import (
	"log"
	"strings"
)

const (
	// This is the prefix we use for queries that are internal to Serf.
	// They are handled internally, and not forwarded to a client.
	InternalQueryPrefix = "_serf_"

	// pingQuery is run to check for reachability
	pingQuery = "ping"

	// conflictQuery is run to resolve a name conflict
	conflictQuery = "conflict"

	// rotateKeyQuery is used to initiate key rotation
	rotateKeyQuery = "rotate-key"
)

// internalQueryName is used to generate a query name for an internal query
func internalQueryName(name string) string {
	return InternalQueryPrefix + name
}

// serfQueries is used to listen for queries that start with
// _serf and respond to them as appropriate.
type serfQueries struct {
	inCh       chan Event
	logger     *log.Logger
	outCh      chan<- Event
	serf       *Serf
	shutdownCh <-chan struct{}
}

// newSerfQueries is used to create a new serfQueries. We return an event
// channel that is ingested and forwarded to an outCh. Any Queries that
// have the InternalQueryPrefix are handled instead of forwarded.
func newSerfQueries(serf *Serf, logger *log.Logger, outCh chan<- Event, shutdownCh <-chan struct{}) (chan<- Event, error) {
	inCh := make(chan Event, 1024)
	q := &serfQueries{
		inCh:       inCh,
		logger:     logger,
		outCh:      outCh,
		serf:       serf,
		shutdownCh: shutdownCh,
	}
	go q.stream()
	return inCh, nil
}

// stream is a long running routine to ingest the event stream
func (s *serfQueries) stream() {
	for {
		select {
		case e := <-s.inCh:
			// Check if this is a query we should process
			if q, ok := e.(*Query); ok && strings.HasPrefix(q.Name, InternalQueryPrefix) {
				go s.handleQuery(q)

			} else if s.outCh != nil {
				s.outCh <- e
			}

		case <-s.shutdownCh:
			return
		}
	}
}

// handleQuery is invoked when we get an internal query
func (s *serfQueries) handleQuery(q *Query) {
	// Get the queryName after the initial prefix
	queryName := q.Name[len(InternalQueryPrefix):]
	switch queryName {
	case pingQuery:
		// Nothing to do, we will ack the query
	case conflictQuery:
		s.handleConflict(q)
	case rotateKeyQuery:
		s.handleRotateKey(q)
	default:
		s.logger.Printf("[WARN] serf: Unhandled internal query '%s'", queryName)
	}
}

// handleConflict is invoked when we get a query that is attempting to
// disambiguate a name conflict. They payload is a node name, and the response
// should the address we believe that node is at, if any.
func (s *serfQueries) handleConflict(q *Query) {
	// The target node name is the payload
	node := string(q.Payload)

	// Do not respond to the query if it is about us
	if node == s.serf.config.NodeName {
		return
	}
	s.logger.Printf("[DEBUG] serf: Got conflict resolution query for '%s'", node)

	// Look for the member info
	var out *Member
	s.serf.memberLock.Lock()
	if member, ok := s.serf.members[node]; ok {
		out = &member.Member
	}
	s.serf.memberLock.Unlock()

	// Encode the response
	buf, err := encodeMessage(messageConflictResponseType, out)
	if err != nil {
		s.logger.Printf("[ERR] serf: Failed to encode conflict query response: %v", err)
		return
	}

	// Send our answer
	if err := q.Respond(buf); err != nil {
		s.logger.Printf("[ERR] serf: Failed to respond to conflict query: %v", err)
	}
}

// handleRotateKey is executed when an internal query to change out the
// encryption key is received.
func (s *serfQueries) handleRotateKey(q *Query) {
	newKey := string(q.Payload)
	s.logger.Printf("[INFO] serf: rotating key from %v to %v", s.serf.config.EncryptKey, newKey)
	s.serf.config.EncryptKey = newKey
}
