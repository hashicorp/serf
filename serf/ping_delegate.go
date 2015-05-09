package serf

import (
	"bytes"
	"log"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/coordinate"
)

const (
	PING_VERSION = 1
)

type pingDelegate struct {
	serf *Serf
}

func (p *pingDelegate) AckPayload() []byte {
	var buf bytes.Buffer

	// Write the version number
	version := []byte{PING_VERSION}
	buf.Write(version)

	enc := codec.NewEncoder(&buf, &codec.MsgpackHandle{})
	if err := enc.Encode(p.serf.coord.GetCoordinate()); err != nil {
		log.Printf("[ERR] serf: Failed to encode coordinate: %v\n", err)
	}
	return buf.Bytes()
}

func (p *pingDelegate) NotifyPingComplete(other *memberlist.Node, rtt time.Duration, payload []byte) {
	if payload == nil || len(payload) == 0 {
		return
	}

	// Verify ping version
	if payload[0] != PING_VERSION {
		log.Printf("[ERR] Unsupported ping version: %v", payload[0])
		return
	}

	var coord coordinate.Coordinate
	r := bytes.NewReader(payload[1:]) // exclude the first byte which is a version number
	dec := codec.NewDecoder(r, &codec.MsgpackHandle{})
	if err := dec.Decode(&coord); err != nil {
		log.Printf("[ERR] serf: Failed to decode coordinate: %v", err)
	}
	if err := p.serf.coord.Update(&coord, rtt); err != nil {
		log.Print(err)
	}

	// Cache the coordinate if the relevant option is set to true
	if p.serf.config.CacheCoordinates {
		p.serf.coordCacheLock.Lock()
		defer p.serf.coordCacheLock.Unlock()
		p.serf.coordCache[other.Name] = &coord
	}
}
