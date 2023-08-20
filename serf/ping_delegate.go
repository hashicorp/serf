// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/coordinate"
)

// pingDelegate is notified when memberlist successfully completes a direct ping
// of a peer node. We use this to update our estimated network coordinate, as
// well as cache the coordinate of the peer.
type pingDelegate struct {
	serf *Serf
}

const (
	// PingVersion is an internal version for the ping message, above the normal
	// versioning we get from the protocol version. This enables small updates
	// to the ping message without a full protocol bump.
	PingVersion = 1
)

// AckPayload is called to produce a payload to send back in response to a ping
// request.
func (p *pingDelegate) AckPayload() []byte {
	var buf bytes.Buffer

	// The first byte is the version number, forming a simple header.
	version := []byte{PingVersion}
	buf.Write(version)

	// The rest of the message is the serialized coordinate.
	enc := codec.NewEncoder(&buf, &codec.MsgpackHandle{})
	if err := enc.Encode(p.serf.coordClient.GetCoordinate()); err != nil {
		p.serf.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to encode coordinate", slog.String("error", err.Error()))
	}
	return buf.Bytes()
}

// NotifyPingComplete is called when this node successfully completes a direct ping
// of a peer node.
func (p *pingDelegate) NotifyPingComplete(other *memberlist.Node, rtt time.Duration, payload []byte) {
	if len(payload) == 0 {
		return
	}

	// Verify ping version in the header.
	version := payload[0]
	if version != PingVersion {
		p.serf.logger.LogAttrs(context.TODO(), slog.LevelError, "Unsupported ping version", slog.Int("version", int(version)))
		return
	}

	// Process the remainder of the message as a coordinate.
	r := bytes.NewReader(payload[1:])
	dec := codec.NewDecoder(r, &codec.MsgpackHandle{})
	var coord coordinate.Coordinate
	if err := dec.Decode(&coord); err != nil {
		p.serf.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to decode coordinate from ping", slog.String("error", err.Error()))
		return
	}

	// Apply the update.
	before := p.serf.coordClient.GetCoordinate()
	after, err := p.serf.coordClient.Update(other.Name, &coord, rtt)
	if err != nil {
		metrics.IncrCounterWithLabels([]string{"serf", "coordinate", "rejected"}, 1, p.serf.metricLabels)
		p.serf.logger.LogAttrs(context.TODO(), slog.LevelDebug, "Rejected coordinate",
			slog.String("from", other.Name), slog.String("error", err.Error()))
		return
	}

	// Publish some metrics to give us an idea of how much we are
	// adjusting each time we update.
	d := float32(before.DistanceTo(after).Seconds() * 1.0e3)
	metrics.AddSampleWithLabels([]string{"serf", "coordinate", "adjustment-ms"}, d, p.serf.metricLabels)

	// Cache the coordinate for the other node, and add our own
	// to the cache as well since it just got updated. This lets
	// users call GetCachedCoordinate with our node name, which is
	// more friendly.
	p.serf.coordCacheLock.Lock()
	p.serf.coordCache[other.Name] = &coord
	p.serf.coordCache[p.serf.config.NodeName] = p.serf.coordClient.GetCoordinate()
	p.serf.coordCacheLock.Unlock()
}
