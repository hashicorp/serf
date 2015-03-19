package serf

import (
	"bytes"
	"log"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/coordinate"
)

type pingDelegate struct {
	serf *Serf
}

func (self *pingDelegate) AckPayload() []byte {
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, &codec.MsgpackHandle{})
	if err := enc.Encode(self.serf.coord); err != nil {
		log.Printf("[ERR] serf: Failed to encode coordinate: %v\n", err)
	}
	return buf.Bytes()
}

func (self *pingDelegate) NotifyPingComplete(other *memberlist.Node, rtt time.Duration, payload []byte) {
	if payload == nil || len(payload) == 0 {
		return
	}

	var coord coordinate.Client
	r := bytes.NewReader(payload)
	dec := codec.NewDecoder(r, &codec.MsgpackHandle{})
	if err := dec.Decode(&coord); err != nil {
		log.Printf("[ERR] serf: Failed to decode coordinate: %v", err)
	}
	if err := self.serf.coord.Update(&coord, rtt); err != nil {
		log.Print(err)
	}
}
