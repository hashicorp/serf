// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"log/slog"

	"github.com/hashicorp/logutils"
)

// logStream is used to stream logs to a client over IPC
type logStream struct {
	client streamClient
	filter *logutils.LevelFilter
	logCh  chan string
	logger *slog.Logger
	seq    uint64
}

func newLogStream(client streamClient, filter *logutils.LevelFilter,
	seq uint64, logger *slog.Logger) *logStream {
	ls := &logStream{
		client: client,
		filter: filter,
		logCh:  make(chan string, 512),
		logger: logger.WithGroup("agent.ipc"),
		seq:    seq,
	}
	go ls.stream()
	return ls
}

func (ls *logStream) HandleLog(l string) {
	// Check the log level
	if !ls.filter.Check([]byte(l)) {
		return
	}

	// Do a non-blocking send
	select {
	case ls.logCh <- l:
	default:
		// We can't log syncronously, since we are already being invoked
		// from the logWriter, and a log will need to invoke Write() which
		// already holds the lock. We must therefor do the log async, so
		// as to not deadlock
		go ls.logger.LogAttrs(context.TODO(), slog.LevelWarn, "Dropping logs", slog.Any("client", ls.client))
	}
}

func (ls *logStream) Stop() {
	close(ls.logCh)
}

func (ls *logStream) stream() {
	header := responseHeader{Seq: ls.seq, Error: ""}
	rec := logRecord{Log: ""}

	for line := range ls.logCh {
		rec.Log = line
		if err := ls.client.Send(&header, &rec); err != nil {
			ls.logger.LogAttrs(context.TODO(), slog.LevelError, " Failed to stream log",
				slog.Any("client", ls.client), slog.String("error", err.Error()))
			return
		}
	}
}
