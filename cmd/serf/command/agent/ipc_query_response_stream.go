// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/hashicorp/serf/serf"
)

// queryResponseStream is used to stream the query results back to a client
type queryResponseStream struct {
	client streamClient
	logger *slog.Logger
	seq    uint64
}

func newQueryResponseStream(client streamClient, seq uint64, logger *slog.Logger) *queryResponseStream {
	qs := &queryResponseStream{
		client: client,
		logger: logger,
		seq:    seq,
	}
	return qs
}

// Stream is a long running routine used to stream the results of a query back to a client
func (qs *queryResponseStream) Stream(resp *serf.QueryResponse) {
	// Setup a timer for the query ending
	remaining := time.Until(resp.Deadline())
	done := time.After(remaining)

	ackCh := resp.AckCh()
	respCh := resp.ResponseCh()
	for {
		select {
		case a := <-ackCh:
			if err := qs.sendAck(a); err != nil {
				qs.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to stream ack to %v: %v", slog.Any("client", qs.client), slog.String("error", err.Error()))
				return
			}
		case r := <-respCh:
			if err := qs.sendResponse(r.From, r.Payload); err != nil {
				qs.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to stream response to %v: %v", slog.Any("client", qs.client), slog.String("error", err.Error()))
				return
			}
		case <-done:
			if err := qs.sendDone(); err != nil {
				qs.logger.LogAttrs(context.TODO(), slog.LevelError, "Failed to stream query end to %v: %v", slog.Any("client", qs.client), slog.String("error", err.Error()))
			}
			return
		}
	}
}

// sendAck is used to send a single ack
func (qs *queryResponseStream) sendAck(from string) error {
	header := responseHeader{
		Seq:   qs.seq,
		Error: "",
	}
	rec := queryRecord{
		Type: queryRecordAck,
		From: from,
	}
	return qs.client.Send(&header, &rec)
}

// sendResponse is used to send a single response
func (qs *queryResponseStream) sendResponse(from string, payload []byte) error {
	header := responseHeader{
		Seq:   qs.seq,
		Error: "",
	}
	rec := queryRecord{
		Type:    queryRecordResponse,
		From:    from,
		Payload: payload,
	}
	return qs.client.Send(&header, &rec)
}

// sendDone is used to signal the end
func (qs *queryResponseStream) sendDone() error {
	header := responseHeader{
		Seq:   qs.seq,
		Error: "",
	}
	rec := queryRecord{
		Type: queryRecordDone,
	}
	return qs.client.Send(&header, &rec)
}
