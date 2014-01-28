package client

import (
	"github.com/hashicorp/serf/serf"
	"net"
)

const (
	maxIPCVersion = 1
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
	Tags        map[string]string
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
