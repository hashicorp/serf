package serf

import (
	"encoding/base64"
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

	// installKeyQuery is used to install a new key
	installKeyQuery = "install-key"

	// useKeyQuery is used to change the primary encryption key
	useKeyQuery = "use-key"

	// removeKeyQuery is used to remove a key from the keyring
	removeKeyQuery = "remove-key"

	// listKeysQuery is used to list all known keys in the cluster
	listKeysQuery = "list-keys"
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

// nodeKeyResponse is used to store the result from an individual node while
// replying to key modification queries
type nodeKeyResponse struct {
	// Result indicates true/false if there were errors or not
	Result bool

	// Message contains error messages or other information
	Message string

	// Keys is used in listing queries to relay a list of installed keys
	Keys []string
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
	case installKeyQuery:
		s.handleInstallKey(q)
	case useKeyQuery:
		s.handleUseKey(q)
	case removeKeyQuery:
		s.handleRemoveKey(q)
	case listKeysQuery:
		s.handleListKeys(q)
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

// handleInstallKey is invoked whenever a new encryption key is received from
// another member in the cluster, and handles the process of installing it onto
// the memberlist keyring. This type of query may fail if the provided key does
// not fit the constraints that memberlist enforces. If the query fails, the
// response will contain the error message so that it may be relayed.
func (s *serfQueries) handleInstallKey(q *Query) {
	response := nodeKeyResponse{Result: false}
	keyring := s.serf.config.MemberlistConfig.Keyring

	if !s.serf.EncryptionEnabled() {
		response.Message = "No keyring to modify (encryption not enabled)"
		s.logger.Printf("[ERR] serf: No keyring to modify (encryption not enabled)")
		goto SEND
	}

	s.logger.Printf("[DEBUG] serf: Received install-key query")
	if err := keyring.AddKey(q.Payload); err != nil {
		response.Message = err.Error()
		s.logger.Printf("[ERR] serf: Failed to install key: %s", err)
		goto SEND
	}

	if err := s.serf.WriteKeyringFile(keyring); err != nil {
		response.Message = err.Error()
		s.logger.Printf("[ERR] serf: Failed to write keyring file: %s", err)
		goto SEND
	}

	response.Result = true

SEND:
	buf, err := encodeMessage(messageKeyResponseType, response)
	if err != nil {
		s.logger.Printf("[ERR] serf: Failed to encode install-key response: %v", err)
		return
	}

	if err := q.Respond(buf); err != nil {
		s.logger.Printf("[ERR] serf: Failed to respond to install-key query: %v", err)
		return
	}
}

// handleUseKey is invoked whenever a query is received to mark a different key
// in the internal keyring as the primary key. This type of query may fail due
// to operator error (requested key not in ring), and thus sends error messages
// back in the response.
func (s *serfQueries) handleUseKey(q *Query) {
	response := nodeKeyResponse{Result: false}
	keyring := s.serf.config.MemberlistConfig.Keyring

	if !s.serf.EncryptionEnabled() {
		response.Message = "No keyring to modify (encryption not enabled)"
		s.logger.Printf("[ERR] serf: No keyring to modify (encryption not enabled)")
		goto SEND
	}

	s.logger.Printf("[DEBUG] serf: Received use-key query")
	if err := keyring.UseKey(q.Payload); err != nil {
		response.Message = err.Error()
		s.logger.Printf("[ERR] serf: Failed to change primary key: %s", err)
		goto SEND
	}

	if err := s.serf.WriteKeyringFile(keyring); err != nil {
		response.Message = err.Error()
		s.logger.Printf("[ERR] serf: Failed to write keyring file: %s", err)
		goto SEND
	}

	response.Result = true

SEND:
	buf, err := encodeMessage(messageKeyResponseType, response)
	if err != nil {
		s.logger.Printf("[ERR] serf: Failed to encode use-key response: %v", err)
		return
	}

	if err := q.Respond(buf); err != nil {
		s.logger.Printf("[ERR] serf: Failed to respond to use-key query: %v", err)
		return
	}
}

// handleRemoveKey is invoked when a query is received to remove a particular
// key from the keyring. This type of query can fail if the key requested for
// deletion is currently the primary key in the keyring, so therefore it will
// reply to the query with any relevant errors from the operation.
func (s *serfQueries) handleRemoveKey(q *Query) {
	response := nodeKeyResponse{Result: false}
	keyring := s.serf.config.MemberlistConfig.Keyring

	if !s.serf.EncryptionEnabled() {
		response.Message = "No keyring to modify (encryption not enabled)"
		s.logger.Printf("[ERR] serf: No keyring to modify (encryption not enabled)")
		goto SEND
	}

	s.logger.Printf("[DEBUG] serf: Received remove-key query")
	if err := keyring.RemoveKey(q.Payload); err != nil {
		response.Message = err.Error()
		s.logger.Printf("[ERR] serf: Failed to remove key: %s", err)
		goto SEND
	}

	if err := s.serf.WriteKeyringFile(keyring); err != nil {
		response.Message = err.Error()
		s.logger.Printf("[ERR] serf: Failed to write keyring file: %s", err)
		goto SEND
	}

	response.Result = true

SEND:
	buf, err := encodeMessage(messageKeyResponseType, response)
	if err != nil {
		s.logger.Printf("[ERR] serf: Failed to encode remove-key response: %v", err)
		return
	}

	if err := q.Respond(buf); err != nil {
		s.logger.Printf("[ERR] serf: Failed to respond to remove-key query: %v", err)
		return
	}
}

// handleListKeys is invoked when a query is received to return a list of all
// installed keys the Serf instance knows of. For performance, the keys are
// encoded to base64 on each of the members to remove this burden from the
// node asking for the results.
func (s *serfQueries) handleListKeys(q *Query) {
	response := nodeKeyResponse{Result: false}
	keyring := s.serf.config.MemberlistConfig.Keyring

	if !s.serf.EncryptionEnabled() {
		response.Message = "Keyring is empty (encryption not enabled)"
		s.logger.Printf("[ERR] serf: Keyring is empty (encryption not enabled)")
		goto SEND
	}

	s.logger.Printf("[DEBUG] serf: Received list-keys query")
	for _, keyBytes := range keyring.GetKeys() {
		// Encode the keys before sending the response. This should help take
		// some the burden of doing this off of the asking member.
		key := base64.StdEncoding.EncodeToString(keyBytes)
		response.Keys = append(response.Keys, key)
	}
	response.Result = true

SEND:
	buf, err := encodeMessage(messageKeyResponseType, response)
	if err != nil {
		s.logger.Printf("[ERR] serf: Failed to encode list-keys response: %v", err)
		return
	}

	if err := q.Respond(buf); err != nil {
		s.logger.Printf("[ERR] serf: Failed to respond to list-keys query: %v", err)
		return
	}
}
