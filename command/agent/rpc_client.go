package agent

import (
	"bufio"
	"fmt"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/mapstructure"
	"github.com/ugorji/go/codec"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

var (
	clientClosed = fmt.Errorf("client closed")
)

type rpcResponseTuple struct {
	response interface{}
	err      error
}

type seqListener struct {
	handler func(resp interface{}, err error)
	persist bool
}

// RPCClient is the RPC client to make requests to the agent RPC.
type RPCClient struct {
	seq          int32
	conn         *net.TCPConn
	reader       *bufio.Reader
	writer       *bufio.Writer
	dec          *codec.Decoder
	enc          *codec.Encoder
	writeLock    sync.Mutex
	dispatch     map[int]seqListener
	dispatchLock sync.Mutex
	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// send is used to send an object using the MsgPack encoding. send
// is serialized to prevent write overlaps, while properly buffering.
func (c *RPCClient) send(obj interface{}) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if err := c.enc.Encode(obj); err != nil {
		return err
	}

	if err := c.writer.Flush(); err != nil {
		return err
	}

	return nil
}

// NewRPCClient is used to create a new RPC client given the address.
// This will properly dial, handshake, and start listening
func NewRPCClient(addr string) (*RPCClient, error) {
	// Try to dial to serf
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	// Create the client
	client := &RPCClient{
		seq:        0,
		conn:       conn.(*net.TCPConn),
		reader:     bufio.NewReader(conn),
		writer:     bufio.NewWriter(conn),
		dispatch:   make(map[int]seqListener),
		shutdownCh: make(chan struct{}),
	}
	client.dec = codec.NewDecoder(client.reader, &codec.MsgpackHandle{})
	client.enc = codec.NewEncoder(client.writer, &codec.MsgpackHandle{})

	// Do the initial handshake
	if err := client.handshake(); err != nil {
		return nil, err
	}
	return client, err
}

// StreamHandle is an opaque handle passed to stop to stop streaming
type StreamHandle int

// Close is used to free any resources associated with the client
func (c *RPCClient) Close() error {
	c.shutdownLock.Lock()
	defer c.shutdownLock.Unlock()

	if !c.shutdown {
		c.shutdown = true
		close(c.shutdownCh)
		return c.conn.Close()
	}
	return nil
}

// ForceLeave is used to ask the agent to issue a leave command for
// a given node
func (c *RPCClient) ForceLeave(node string) error {
	req := forceLeaveRequest{
		Command: forceLeaveCommand,
		Seq:     c.getSeq(),
		Node:    node,
	}
	return c.genericRPC(req.Seq, req)
}

// Join is used to instruct the agent to attempt a join
func (c *RPCClient) Join(addrs []string, ignoreOld bool) (n int, err error) {
	req := joinRequest{
		Command:  membersCommand,
		Seq:      c.getSeq(),
		Existing: addrs,
		Replay:   !ignoreOld,
	}
	ch := c.waitSeq(req.Seq)
	if err := c.send(req); err != nil {
		return 0, err
	}

	select {
	case tuple := <-ch:
		if tuple.err != nil {
			return 0, tuple.err
		}

		// Decode the response
		var resp joinResponse
		if err := mapstructure.Decode(tuple.response, &resp); err != nil {
			return 0, err
		}
		return resp.Num, strError(resp.Error)

	case <-c.shutdownCh:
		return 0, clientClosed
	}
}

// Members is used to fetch a list of known members
func (c *RPCClient) Members() ([]serf.Member, error) {
	req := membersRequest{
		Command: membersCommand,
		Seq:     c.getSeq(),
	}
	ch := c.waitSeq(req.Seq)
	if err := c.send(req); err != nil {
		return nil, err
	}

	select {
	case tuple := <-ch:
		// Check for an error
		if tuple.err != nil {
			return nil, tuple.err
		}

		// Decode the response
		var resp membersResponse
		if err := mapstructure.Decode(tuple.response, &resp); err != nil {
			return nil, err
		}
		return resp.Members, nil

	case <-c.shutdownCh:
		return nil, clientClosed
	}
}

// UserEvent is used to trigger sending an event
func (c *RPCClient) UserEvent(name string, payload []byte, coalesce bool) error {
	req := eventRequest{
		Command:  eventCommand,
		Seq:      c.getSeq(),
		Name:     name,
		Payload:  payload,
		Coalesce: coalesce,
	}
	return c.genericRPC(req.Seq, req)
}

// Monitor is used to subscribe to the logs of the agent
func (c *RPCClient) Monitor(level logutils.LogLevel, ch chan<- string) (StreamHandle, error) {
	// Setup the request
	seq := c.getSeq()
	req := monitorRequest{
		Command:  monitorCommand,
		Seq:      seq,
		LogLevel: string(level),
	}
	if err := c.genericRPC(req.Seq, req); err != nil {
		return 0, err
	}

	// Create a handler
	handler := func(resp interface{}, err error) {
		resp_map, ok := resp.(map[string]interface{})
		if !ok {
			return
		}
		raw, ok := getField(resp_map, "Log")
		if !ok {
			return
		}
		if log, ok := raw.(string); ok {
			ch <- log
		}
	}

	// Register the handler
	c.dispatchLock.Lock()
	c.dispatch[seq] = seqListener{handler: handler, persist: true}
	c.dispatchLock.Unlock()

	// Return a handle
	return StreamHandle(seq), nil
}

// Stream is used to subscribe to events
func (c *RPCClient) Stream(filter string, ch chan<- map[string]interface{}) (StreamHandle, error) {
	// Setup the request
	seq := c.getSeq()
	req := streamRequest{
		Command: streamCommand,
		Seq:     seq,
		Type:    filter,
	}
	if err := c.genericRPC(req.Seq, req); err != nil {
		return 0, err
	}

	// Create a handler
	handler := func(resp interface{}, err error) {
		resp_map, ok := resp.(map[string]interface{})
		if !ok {
			return
		}
		ch <- resp_map
	}

	// Register the handler
	c.dispatchLock.Lock()
	c.dispatch[seq] = seqListener{handler: handler, persist: true}
	c.dispatchLock.Unlock()

	// Return a handle
	return StreamHandle(seq), nil
}

// Stop is used to unsubscribe from logs or event streams
func (c *RPCClient) Stop(handle StreamHandle) error {
	// Deregister locally first to stop delivery
	c.dispatchLock.Lock()
	delete(c.dispatch, int(handle))
	c.dispatchLock.Unlock()

	req := stopRequest{
		Command: stopCommand,
		Seq:     c.getSeq(),
		Stop:    int(handle),
	}
	return c.genericRPC(req.Seq, req)
}

// handshake is used to perform the initial handshake on connect
func (c *RPCClient) handshake() error {
	req := handshakeRequest{
		Command: handshakeCommand,
		Seq:     c.getSeq(),
		Version: MaxIPCVersion,
	}
	return c.genericRPC(req.Seq, req)
}

// genericRPC is used to send a request and wait for an
// errorSequenceResponse, potentially returning an error
func (c *RPCClient) genericRPC(seq int, req interface{}) error {
	ch := c.waitSeq(seq)
	if err := c.send(req); err != nil {
		return err
	}
	select {
	case tuple := <-ch:
		if tuple.err != nil {
			return tuple.err
		}
		return c.maybeError(tuple.response)
	case <-c.shutdownCh:
		return clientClosed
	}
}

// maybeError is used for errorSeqResponse, where we may have an error
func (c *RPCClient) maybeError(resp interface{}) error {
	var errSeq errorSeqResponse
	if err := mapstructure.Decode(resp, &errSeq); err != nil {
		return err
	}
	return strError(errSeq.Error)
}

// strError converts a string to an error if not blank
func strError(s string) error {
	if s != "" {
		return fmt.Errorf(s)
	}
	return nil
}

// getSeq returns the next sequence number in a safe manner
func (c *RPCClient) getSeq() int {
	return int(atomic.AddInt32(&c.seq, 1))
}

// waitSeq is used to setup a channel to wait on a response for
// a given sequence number.
func (c *RPCClient) waitSeq(seq int) chan rpcResponseTuple {
	c.dispatchLock.Lock()
	defer c.dispatchLock.Unlock()

	ch := make(chan rpcResponseTuple, 1)
	handler := func(resp interface{}, err error) {
		ch <- rpcResponseTuple{resp, err}
	}

	c.dispatch[seq] = seqListener{handler: handler, persist: false}
	return ch
}

// respondSeq is used to respond to a given sequence number
func (c *RPCClient) respondSeq(seq int, resp interface{}, err error) {
	c.dispatchLock.Lock()
	defer c.dispatchLock.Unlock()

	seqL, ok := c.dispatch[seq]
	if !ok {
		return
	}

	seqL.handler(resp, nil)
	if !seqL.persist {
		delete(c.dispatch, seq)
	}
}

// listen is used to processes data coming over the IPC channel,
// and wrote it to the correct destination based on seq no
func (c *RPCClient) listen() {
	defer c.Close()
	var resp map[string]interface{}
	for {
		resp = nil
		if err := c.dec.Decode(&resp); err != nil {
			log.Printf("[ERR] agent.client: Failed to decode client response: %v", err)
			break
		}

		// Look for the seq
		seq_raw, ok := getField(resp, "Seq")
		if !ok {
			log.Printf("[ERR] agent.client: Response missing Seq: %#v", resp)
			continue
		}

		// Try to convert
		seq, ok := seq_raw.(int)
		if !ok {
			log.Printf("[ERR] agent.client: Seq not int: %#v", seq_raw)
			continue
		}

		// Try to dispatch
		c.respondSeq(seq, resp, nil)
	}
}
