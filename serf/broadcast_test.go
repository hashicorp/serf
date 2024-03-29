// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
)

func TestBroadcast_Impl(t *testing.T) {
	var _ memberlist.Broadcast = &broadcast{}
}

func TestBroadcastFinished(t *testing.T) {
	t.Parallel()

	ch := make(chan struct{})
	b := &broadcast{notify: ch}
	b.Finished()

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		t.Fatalf("should have notified")
	}
}

func TestBroadcastFinished_nilNotify(t *testing.T) {
	t.Parallel()

	b := &broadcast{notify: nil}
	b.Finished()
}
