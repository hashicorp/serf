package agent

/*
 The agent exposes an IPC mechanism that is used for both controlling
 Serf as well as providing a fast streaming mechanism for events. This
 allows other applications to easily leverage Serf as the event layer.

 We additionally make use of the IPC layer to also handle RPC calls from
 the CLI to unify the code paths. This results in a split Request/Response
 as well as streaming mode of operation.

 The system is fairly simple, each client opens a TCP connection to the
 agent. The connection is initialized with a handshake which establishes
 the protocol version being used. This is to allow for future changes to
 the protocol.

 Once initialized, clients send commands and wait for responses. Certain
 commands will cause the client to subscribe to events, and those will be
 pushed down the socket as they are received. This provides a low-latency
 mechanism for applications to send and receive events, while also providing
 a flexible control mechanism for Serf.
*/

import (
	"bufio"
	"fmt"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/serf"
	"github.com/ugorji/go/codec"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

const (
	MinIPCVersion = 1
	MaxIPCVersion = 1
)

const (
	handshakeCommand  = "handshake"
	eventCommand      = "event"
	forceLeaveCommand = "force-leave"
	joinCommand       = "join"
	membersCommand    = "members"
	streamCommand     = "stream"
	stopCommand       = "stop"
	monitorCommand    = "monitor"
	leaveCommand      = "leave"
)

const (
	unsupportedCommand    = "Unsupported command"
	unsupportedIPCVersion = "Unsupported IPC version"
	duplicateHandshake    = "Handshake already performed"
	handshakeRequired     = "Handshake required"
	monitorExists         = "Monitor already exists"
	invalidFilter         = "Invalid event filter"
	streamExists          = "Stream with given sequence exists"
)

// Request header is sent before each request
type requestHeader struct {
	Command string
	Seq     uint64
}

// Response header is sent before each response
type responseHeader struct {
	Seq   uint64
	Error string
}

type handshakeRequest struct {
	Version int32
}

type eventRequest struct {
	Name     string
	Payload  []byte
	Coalesce bool
}

type forceLeaveRequest struct {
	Node string
}

type joinRequest struct {
	Existing []string
	Replay   bool
}

type joinResponse struct {
	Num int32
}

type membersResponse struct {
	Members []Member
}

type monitorRequest struct {
	LogLevel string
}

type streamRequest struct {
	Type string
}

type stopRequest struct {
	Stop uint64
}

type logRecord struct {
	Log string
}

type userEventRecord struct {
	Event    string
	LTime    serf.LamportTime
	Name     string
	Payload  []byte
	Coalesce bool
}

type Member struct {
	Name        string
	Addr        net.IP
	Port        uint16
	Role        string
	Status      string
	ProtocolMin uint8
	ProtocolMax uint8
	ProtocolCur uint8
	DelegateMin uint8
	DelegateMax uint8
	DelegateCur uint8
}

type memberEventRecord struct {
	Event   string
	Members []Member
}

type AgentIPC struct {
	sync.Mutex
	agent     *Agent
	clients   map[string]*IPCClient
	listener  net.Listener
	logger    *log.Logger
	logWriter *logWriter
	stop      bool
	stopCh    chan struct{}
}

type IPCClient struct {
	name         string
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	dec          *codec.Decoder
	enc          *codec.Encoder
	writeLock    sync.Mutex
	version      int32 // From the handshake, 0 before
	logStreamer  *logStream
	eventStreams map[uint64]*eventStream
}

// send is used to send an object using the MsgPack encoding. send
// is serialized to prevent write overlaps, while properly buffering.
func (c *IPCClient) Send(header *responseHeader, obj interface{}) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	if err := c.enc.Encode(header); err != nil {
		return err
	}

	if obj != nil {
		if err := c.enc.Encode(obj); err != nil {
			return err
		}
	}

	if err := c.writer.Flush(); err != nil {
		return err
	}

	return nil
}

func (c *IPCClient) String() string {
	return fmt.Sprintf("ipc.client: %v", c.conn)
}

// NewAgentIPC is used to create a new Agent IPC handler
func NewAgentIPC(agent *Agent, listener net.Listener,
	logOutput io.Writer, logWriter *logWriter) *AgentIPC {
	if logOutput == nil {
		logOutput = os.Stderr
	}
	ipc := &AgentIPC{
		agent:     agent,
		clients:   make(map[string]*IPCClient),
		listener:  listener,
		logger:    log.New(logOutput, "", log.LstdFlags),
		logWriter: logWriter,
		stopCh:    make(chan struct{}),
	}
	go ipc.listen()
	return ipc
}

// Shutdown is used to shutdown the IPC layer
func (i *AgentIPC) Shutdown() {
	i.Lock()
	defer i.Unlock()

	if i.stop {
		return
	}

	i.stop = true
	close(i.stopCh)
	i.listener.Close()

	// Close the existing connections
	for _, client := range i.clients {
		client.conn.Close()
	}
}

// listen is a long running routine that listens for new clients
func (i *AgentIPC) listen() {
	for {
		conn, err := i.listener.Accept()
		if err != nil {
			if i.stop {
				return
			}
			i.logger.Printf("[ERR] agent.ipc: Failed to accept client: %v", err)
			continue
		}
		i.logger.Printf("[INFO] agent.ipc: Accepted client: %v", conn.RemoteAddr())

		// Wrap the connection in a client
		client := &IPCClient{
			name:         conn.RemoteAddr().String(),
			conn:         conn,
			reader:       bufio.NewReader(conn),
			writer:       bufio.NewWriter(conn),
			eventStreams: make(map[uint64]*eventStream),
		}
		client.dec = codec.NewDecoder(client.reader,
			&codec.MsgpackHandle{RawToString: true, WriteExt: true})
		client.enc = codec.NewEncoder(client.writer,
			&codec.MsgpackHandle{RawToString: true, WriteExt: true})
		if err != nil {
			i.logger.Printf("[ERR] agent.ipc: Failed to create decoder: %v", err)
			conn.Close()
			continue
		}

		// Register the client
		i.Lock()
		if !i.stop {
			i.clients[client.name] = client
			go i.handleClient(client)
		} else {
			conn.Close()
		}
		i.Unlock()
	}
}

// deregisterClient is called to cleanup after a client disconnects
func (i *AgentIPC) deregisterClient(client *IPCClient) {
	// Close the socket
	client.conn.Close()

	// Remove from the clients list
	i.Lock()
	delete(i.clients, client.name)
	i.Unlock()

	// Remove from the log writer
	if client.logStreamer != nil {
		i.logWriter.DeregisterHandler(client.logStreamer)
		client.logStreamer.Stop()
	}

	// Remove from event handlers
	for _, es := range client.eventStreams {
		i.agent.DeregisterEventHandler(es)
		es.Stop()
	}
}

// handleClient is a long running routine that handles a single client
func (i *AgentIPC) handleClient(client *IPCClient) {
	defer i.deregisterClient(client)
	var reqHeader requestHeader
	for {
		// Decode the header
		if err := client.dec.Decode(&reqHeader); err != nil {
			if err != io.EOF && !i.stop {
				i.logger.Printf("[ERR] agent.ipc: failed to decode request header: %v", err)
			}
			return
		}

		// Evaluate the command
		if err := i.handleRequest(client, &reqHeader); err != nil {
			i.logger.Printf("[ERR] agent.ipc: Failed to evaluate request: %v", err)
			return
		}
	}
}

// handleRequest is used to evaluate a single client command
func (i *AgentIPC) handleRequest(client *IPCClient, reqHeader *requestHeader) error {
	// Look for a command field
	command := reqHeader.Command
	seq := reqHeader.Seq

	// Ensure the handshake is performed before other commands
	if command != handshakeCommand && client.version == 0 {
		respHeader := responseHeader{Seq: seq, Error: handshakeRequired}
		client.Send(&respHeader, nil)
		return fmt.Errorf(handshakeRequired)
	}

	// Dispatch command specific handlers
	switch command {
	case handshakeCommand:
		return i.handleHandshake(client, seq)

	case eventCommand:
		return i.handleEvent(client, seq)

	case membersCommand:
		return i.handleMembers(client, seq)

	case streamCommand:
		return i.handleStream(client, seq)

	case monitorCommand:
		return i.handleMonitor(client, seq)

	case stopCommand:
		return i.handleStop(client, seq)

	case forceLeaveCommand:
		return i.handleForceLeave(client, seq)

	case joinCommand:
		return i.handleJoin(client, seq)

	case leaveCommand:
		return i.handleLeave(client, seq)

	default:
		respHeader := responseHeader{Seq: seq, Error: unsupportedCommand}
		client.Send(&respHeader, nil)
		return fmt.Errorf("command '%s' not recognized", command)
	}
}

func (i *AgentIPC) handleHandshake(client *IPCClient, seq uint64) error {
	var req handshakeRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := responseHeader{
		Seq:   seq,
		Error: "",
	}

	// Check the version
	if req.Version < MinIPCVersion || req.Version > MaxIPCVersion {
		resp.Error = unsupportedIPCVersion
	} else if client.version != 0 {
		resp.Error = duplicateHandshake
	} else {
		client.version = req.Version
	}
	return client.Send(&resp, nil)
}

func (i *AgentIPC) handleEvent(client *IPCClient, seq uint64) error {
	var req eventRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	// Attempt the send
	err := i.agent.UserEvent(req.Name, req.Payload, req.Coalesce)

	// Respond
	resp := responseHeader{
		Seq:   seq,
		Error: errToString(err),
	}
	return client.Send(&resp, nil)
}

func (i *AgentIPC) handleForceLeave(client *IPCClient, seq uint64) error {
	var req forceLeaveRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	// Attempt leave
	err := i.agent.ForceLeave(req.Node)

	// Respond
	resp := responseHeader{
		Seq:   seq,
		Error: errToString(err),
	}
	return client.Send(&resp, nil)
}

func (i *AgentIPC) handleJoin(client *IPCClient, seq uint64) error {
	var req joinRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	// Attempt the join
	num, err := i.agent.Join(req.Existing, req.Replay)

	// Respond
	header := responseHeader{
		Seq:   seq,
		Error: errToString(err),
	}
	resp := joinResponse{
		Num: int32(num),
	}
	return client.Send(&header, &resp)
}

func (i *AgentIPC) handleMembers(client *IPCClient, seq uint64) error {
	serf := i.agent.Serf()
	raw := serf.Members()

	members := make([]Member, 0, len(raw))
	for _, m := range raw {
		sm := Member{
			Name:        m.Name,
			Addr:        m.Addr,
			Port:        m.Port,
			Role:        m.Role,
			Status:      m.Status.String(),
			ProtocolMin: m.ProtocolMin,
			ProtocolMax: m.ProtocolMax,
			ProtocolCur: m.ProtocolCur,
			DelegateMin: m.DelegateMin,
			DelegateMax: m.DelegateMax,
			DelegateCur: m.DelegateCur,
		}
		members = append(members, sm)
	}

	header := responseHeader{
		Seq:   seq,
		Error: "",
	}
	resp := membersResponse{
		Members: members,
	}
	return client.Send(&header, &resp)
}

func (i *AgentIPC) handleStream(client *IPCClient, seq uint64) error {
	var es *eventStream
	var req streamRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := responseHeader{
		Seq:   seq,
		Error: "",
	}

	// Create the event filters
	filters := ParseEventFilter(req.Type)
	for _, f := range filters {
		if !f.Valid() {
			resp.Error = invalidFilter
			goto SEND
		}
	}

	// Check if there is an existing stream
	if _, ok := client.eventStreams[seq]; ok {
		resp.Error = streamExists
		goto SEND
	}

	// Create an event streamer
	es = newEventStream(client, filters, seq, i.logger)
	client.eventStreams[seq] = es

	// Register with the agent. Defer so that we can respond before
	// registration, avoids any possible race condition
	defer i.agent.RegisterEventHandler(es)

SEND:
	return client.Send(&resp, nil)
}

func (i *AgentIPC) handleMonitor(client *IPCClient, seq uint64) error {
	var req monitorRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := responseHeader{
		Seq:   seq,
		Error: "",
	}

	// Upper case the log level
	req.LogLevel = strings.ToUpper(req.LogLevel)

	// Create a level filter
	filter := LevelFilter()
	filter.MinLevel = logutils.LogLevel(req.LogLevel)
	if !ValidateLevelFilter(filter.MinLevel, filter) {
		resp.Error = fmt.Sprintf("Unknown log level: %s", filter.MinLevel)
		goto SEND
	}

	// Check if there is an existing monitor
	if client.logStreamer != nil {
		resp.Error = monitorExists
		goto SEND
	}

	// Create a log streamer
	client.logStreamer = newLogStream(client, filter, seq, i.logger)

	// Register with the log writer. Defer so that we can respond before
	// registration, avoids any possible race condition
	defer i.logWriter.RegisterHandler(client.logStreamer)

SEND:
	return client.Send(&resp, nil)
}

func (i *AgentIPC) handleStop(client *IPCClient, seq uint64) error {
	var req stopRequest
	if err := client.dec.Decode(&req); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	// Remove a log monitor if any
	if client.logStreamer != nil && client.logStreamer.seq == req.Stop {
		i.logWriter.DeregisterHandler(client.logStreamer)
		client.logStreamer.Stop()
		client.logStreamer = nil
	}

	// Remove an event stream if any
	if es, ok := client.eventStreams[req.Stop]; ok {
		i.agent.DeregisterEventHandler(es)
		es.Stop()
		delete(client.eventStreams, req.Stop)
	}

	// Always succeed
	resp := responseHeader{Seq: seq, Error: ""}
	return client.Send(&resp, nil)
}

func (i *AgentIPC) handleLeave(client *IPCClient, seq uint64) error {
	i.logger.Printf("[INFO] agent.ipc: Graceful leave triggered")

	// Do the leave
	err := i.agent.Leave()
	if err != nil {
		i.logger.Printf("[ERR] agent.ipc: leave failed: %v", err)
	}
	resp := responseHeader{Seq: seq, Error: errToString(err)}

	// Send and wait
	err = client.Send(&resp, nil)

	// Trigger a shutdown!
	if err := i.agent.Shutdown(); err != nil {
		i.logger.Printf("[ERR] agent.ipc: shutdown failed: %v", err)
	}
	return err
}

// Used to convert an error to a string representation
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
