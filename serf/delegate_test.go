// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"reflect"
	"testing"

	"github.com/hashicorp/serf/testutil"
)

func TestDelegate_NodeMeta_Old(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	c.ProtocolVersion = 2
	c.Tags["role"] = "test"
	d := &delegate{&Serf{config: c}}
	meta := d.NodeMeta(32)

	if !reflect.DeepEqual(meta, []byte("test")) {
		t.Fatalf("bad meta data: %v", meta)
	}

	out := d.serf.decodeTags(meta)
	if out["role"] != "test" {
		t.Fatalf("bad meta data: %v", meta)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	d.NodeMeta(1)
}

func TestDelegate_NodeMeta_New(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	c.ProtocolVersion = 3
	c.Tags["role"] = "test"
	d := &delegate{&Serf{config: c}}
	meta := d.NodeMeta(32)

	out := d.serf.decodeTags(meta)
	if out["role"] != "test" {
		t.Fatalf("bad meta data: %v", meta)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	d.NodeMeta(1)
}

// internals
func TestDelegate_LocalState(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	c1 := testConfig(t, ip1)
	s1, err := Create(c1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	c2 := testConfig(t, ip2)
	s2, err := Create(c2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2)

	_, err = s1.Join([]string{c2.NodeName + "/" + c2.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)

	err = s1.UserEvent("test", []byte("test"), false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = s1.Query("foo", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// s2 can leave now
	if err = s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do a state dump
	d := c1.MemberlistConfig.Delegate
	buf := d.LocalState(false)

	// Verify
	if messageType(buf[0]) != messagePushPullType {
		t.Fatalf("bad message type")
	}

	// Attempt a decode
	pp := messagePushPull{}
	if err := decodeMessage(buf[1:], &pp); err != nil {
		t.Fatalf("decode failed %v", err)
	}

	// Verify lamport
	if pp.LTime != s1.clock.Time() {
		t.Fatalf("clock mismatch")
	}

	// Verify the status
	// Leave waits until propagation so this should only have one member
	if len(pp.StatusLTimes) != 1 {
		t.Fatalf("missing ltimes")
	}

	if len(pp.LeftMembers) != 0 {
		t.Fatalf("should have no left members")
	}

	if pp.EventLTime != s1.eventClock.Time() {
		t.Fatalf("clock mismatch")
	}

	if len(pp.Events) != s1.config.EventBuffer {
		t.Fatalf("should send full event buffer")
	}

	if pp.QueryLTime != s1.queryClock.Time() {
		t.Fatalf("clock mismatch")
	}
}

// internals
func TestDelegate_MergeRemoteState(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c1 := testConfig(t, ip1)
	s1, err := Create(c1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	// Do a state dump
	d := c1.MemberlistConfig.Delegate

	// Make a fake push pull
	pp := messagePushPull{
		LTime: 42,
		StatusLTimes: map[string]LamportTime{
			"test": 20,
			"foo":  15,
		},
		LeftMembers: []string{"foo"},
		EventLTime:  50,
		Events: []*userEvents{
			&userEvents{
				LTime: 45,
				Events: []userEvent{
					userEvent{
						Name:    "test",
						Payload: nil,
					},
				},
			},
		},
		QueryLTime: 100,
	}

	buf, err := encodeMessage(messagePushPullType, &pp, c1.MsgpackUseNewTimeFormat)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Merge in fake state
	d.MergeRemoteState(buf, false)

	// Verify lamport
	if s1.clock.Time() != 42 {
		t.Fatalf("clock mismatch")
	}

	// Verify pending join for test
	if join, ok := recentIntent(s1.recentIntents, "test", messageJoinType); !ok || join != 20 {
		t.Fatalf("bad recent join")
	}

	// Verify pending leave for foo
	if leave, ok := recentIntent(s1.recentIntents, "foo", messageLeaveType); !ok || leave != 16 {
		t.Fatalf("bad recent leave")
	}

	// Very event time
	if s1.eventClock.Time() != 50 {
		t.Fatalf("bad event clock")
	}

	if s1.eventBuffer[45] == nil {
		t.Fatalf("missing event buffer for time")
	}
	if s1.eventBuffer[45].Events[0].Name != "test" {
		t.Fatalf("missing event")
	}

	if s1.queryClock.Time() != 100 {
		t.Fatalf("bad query clock")
	}
}

func TestDelegate_BadData(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	c.ProtocolVersion = 3
	c.Tags["role"] = "test"
	d := &delegate{&Serf{config: c}}
	meta := d.NodeMeta(32)

	out := d.serf.decodeTags(meta)
	if out["role"] != "test" {
		t.Fatalf("bad meta data: %v", meta)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	d.NodeMeta(1)
}
