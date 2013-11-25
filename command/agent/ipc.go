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
	"github.com/mitchellh/mapstructure"
	"github.com/ugorji/go/codec"
	"io"
	"log"
	"net"
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

type handshakeRequest struct {
	Command string
	Seq     int
	Version int
}

type eventRequest struct {
	Command  string
	Seq      int
	Name     string
	Payload  []byte
	Coalesce bool
}

type forceLeaveRequest struct {
	Command string
	Seq     int
	Node    string
}

type joinRequest struct {
	Command  string
	Seq      int
	Existing []string
	Replay   bool
}

type joinResponse struct {
	Seq   int
	Error string
	Num   int
}

type membersRequest struct {
	Command string
	Seq     int
}

type membersResponse struct {
	Seq     int
	Members []serf.Member
}

type monitorRequest struct {
	Command  string
	Seq      int
	LogLevel string
}

type streamRequest struct {
	Command string
	Seq     int
	Type    string
}

type stopRequest struct {
	Command string
	Seq     int
}

type errorSeqResponse struct {
	Seq   int
	Error string
}

type logRecord struct {
	Seq int
	Log string
}

type userEventRecord struct {
	Seq      int
	Event    string
	LTime    serf.LamportTime
	Name     string
	Payload  []byte
	Coalesce bool
}

type member struct {
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
	Seq     int
	Event   string
	Members []member
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
	mapstructure.DecoderConfig
	name         string
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	dec          *codec.Decoder
	enc          *codec.Encoder
	writeLock    sync.Mutex
	mapper       *mapstructure.Decoder
	version      int // From the handshake, 0 before
	logStreamer  *logStream
	eventStreams map[int]*eventStream
}

// send is used to send an object using the MsgPack encoding. send
// is serialized to prevent write overlaps, while properly buffering.
func (c *IPCClient) send(obj interface{}) error {
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

// NewAgentIPC is used to create a new Agent IPC handler
func NewAgentIPC(agent *Agent, listener net.Listener,
	logOutput io.Writer, logWriter *logWriter) *AgentIPC {
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

		// Wrap the connection in a client
		client := &IPCClient{
			DecoderConfig: mapstructure.DecoderConfig{
				ErrorUnused: true,
				Result:      &struct{}{},
			},
			name:         conn.RemoteAddr().String(),
			conn:         conn,
			reader:       bufio.NewReader(conn),
			writer:       bufio.NewWriter(conn),
			eventStreams: make(map[int]*eventStream),
		}
		client.dec = codec.NewDecoder(client.reader, &codec.MsgpackHandle{})
		client.enc = codec.NewEncoder(client.writer, &codec.MsgpackHandle{})
		client.mapper, err = mapstructure.NewDecoder(&client.DecoderConfig)
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
	var req map[string]interface{}
	for {
		// Decode the command
		if err := client.dec.Decode(&req); err != nil {
			if err != io.EOF {
				i.logger.Printf("[ERR] agent.ipc: Failed to decode client request: %v", err)
			}
			return
		}

		// Evaluate the command
		if err := i.handleRequest(client, req); err != nil {
			i.logger.Printf("[ERR] agent.ipc: Failed to evaluate client request: %v", err)
			return
		}
	}
}

// getField tries to get a field from a request, checking both the upper
// and lower case variants. The field should be provided as title cased.
func getField(req map[string]interface{}, field string) (interface{}, bool) {
	if val, ok := req[field]; ok {
		return val, ok
	}
	val, ok := req[strings.ToLower(field)]
	return val, ok
}

// handleRequest is used to evaluate a single client command
func (i *AgentIPC) handleRequest(client *IPCClient, req map[string]interface{}) error {
	// Look for a command field
	command_raw, ok := getField(req, "Command")
	if !ok {
		return fmt.Errorf("missing command field: %#v", req)
	}
	command, ok := command_raw.(string)
	if !ok {
		return fmt.Errorf("command field not a string: %#v", req)
	}

	// Try to get the sequence number
	var seq int
	if seq_raw, ok := getField(req, "Seq"); ok {
		seq, ok = seq_raw.(int)
	}

	// Ensure the handshake is performed before other commands
	if command != handshakeCommand && client.version == 0 {
		client.send(&errorSeqResponse{Error: handshakeRequired, Seq: seq})
		return fmt.Errorf(handshakeRequired)
	}

	// Dispatch command specific handlers
	switch command {
	case handshakeCommand:
		return i.handleHandshake(client, req)

	case eventCommand:
		return i.handleEvent(client, req)

	case forceLeaveCommand:
		return i.handleForceLeave(client, req)

	case joinCommand:
		return i.handleJoin(client, req)

	case membersCommand:
		return i.handleMembers(client, req)

	case streamCommand:
		return i.handleStream(client, req)

	case monitorCommand:
		return i.handleMonitor(client, req)

	case stopCommand:
		return i.handleStop(client, req)

	default:
		client.send(&errorSeqResponse{Error: unsupportedCommand, Seq: seq})
		return fmt.Errorf("command '%s' not recognized", command)
	}
}

func (i *AgentIPC) handleHandshake(client *IPCClient, raw map[string]interface{}) error {
	var req handshakeRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := errorSeqResponse{
		Seq:   req.Seq,
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
	return client.send(&resp)
}

func (i *AgentIPC) handleEvent(client *IPCClient, raw map[string]interface{}) error {
	var req eventRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := errorSeqResponse{
		Seq:   req.Seq,
		Error: "",
	}
	if err := i.agent.UserEvent(req.Name, req.Payload, req.Coalesce); err != nil {
		resp.Error = err.Error()
	}
	return client.send(&resp)
}

func (i *AgentIPC) handleForceLeave(client *IPCClient, raw map[string]interface{}) error {
	var req forceLeaveRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := errorSeqResponse{
		Seq:   req.Seq,
		Error: "",
	}
	if err := i.agent.ForceLeave(req.Node); err != nil {
		resp.Error = err.Error()
	}
	return client.send(&resp)
}

func (i *AgentIPC) handleJoin(client *IPCClient, raw map[string]interface{}) error {
	var req joinRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := joinResponse{
		Seq:   req.Seq,
		Error: "",
		Num:   0,
	}
	num, err := i.agent.Join(req.Existing, req.Replay)
	resp.Num = num
	if err != nil {
		resp.Error = err.Error()
	}
	return client.send(&resp)
}

func (i *AgentIPC) handleMembers(client *IPCClient, raw map[string]interface{}) error {
	var req membersRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	serf := i.agent.Serf()
	resp := membersResponse{
		Seq:     req.Seq,
		Members: serf.Members(),
	}
	return client.send(&resp)
}

func (i *AgentIPC) handleStream(client *IPCClient, raw map[string]interface{}) error {
	var es *eventStream
	var req streamRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := errorSeqResponse{
		Seq:   req.Seq,
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
	if _, ok := client.eventStreams[req.Seq]; ok {
		resp.Error = streamExists
		goto SEND
	}

	// Create an event streamer
	es = newEventStream(client, filters, req.Seq, i.logger)
	client.eventStreams[req.Seq] = es

	// Register with the agent
	i.agent.RegisterEventHandler(es)

SEND:
	return client.send(&resp)
}

func (i *AgentIPC) handleMonitor(client *IPCClient, raw map[string]interface{}) error {
	var req monitorRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	resp := errorSeqResponse{
		Seq:   req.Seq,
		Error: "",
	}

	// Create a level filter
	filter := LevelFilter()
	filter.MinLevel = logutils.LogLevel(req.LogLevel)
	if !ValidateLevelFilter(filter) {
		resp.Error = fmt.Sprintf("Unknown log level: %s", filter.MinLevel)
		goto SEND
	}

	// Check if there is an existing monitor
	if client.logStreamer != nil {
		resp.Error = monitorExists
		goto SEND
	}

	// Create a log streamer
	client.logStreamer = newLogStream(client, filter, req.Seq, i.logger)

	// Register with the log writer
	i.logWriter.RegisterHandler(client.logStreamer)

SEND:
	return client.send(&resp)
}

func (i *AgentIPC) handleStop(client *IPCClient, raw map[string]interface{}) error {
	var req stopRequest
	client.Result = &req
	if err := client.mapper.Decode(raw); err != nil {
		return fmt.Errorf("decode failed: %v", err)
	}

	// Remove a log monitor if any
	if client.logStreamer != nil && client.logStreamer.seq == req.Seq {
		i.logWriter.DeregisterHandler(client.logStreamer)
		client.logStreamer.Stop()
		client.logStreamer = nil
	}

	// Remove an event stream if any
	if es, ok := client.eventStreams[req.Seq]; ok {
		i.agent.DeregisterEventHandler(es)
		es.Stop()
		delete(client.eventStreams, req.Seq)
	}

	// Always succeed
	resp := errorSeqResponse{Seq: req.Seq, Error: ""}
	return client.send(&resp)
}
