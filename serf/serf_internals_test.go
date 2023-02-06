// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf/internal/race"
	"github.com/hashicorp/serf/testutil"
	"github.com/hashicorp/serf/testutil/retry"
)

func TestSerf_joinLeave_ltime(t *testing.T) {
	if race.Enabled {
		t.Skip("test contains a data race")
	}

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)
	retry.Run(t, func(r *retry.R) {
		if s2.members[s1.config.NodeName].statusLTime != 1 {
			r.Fatalf("join time is not valid %d",
				s2.members[s1.config.NodeName].statusLTime)
		}

		if s2.clock.Time() <= s2.members[s1.config.NodeName].statusLTime {
			r.Fatalf("join should increment")
		}
	})

	oldClock := s2.clock.Time()

	if err := s1.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		// s1 clock should exceed s2 due to leave
		if s2.clock.Time() <= oldClock {
			r.Fatalf("leave should increment (%d / %d)",
				s2.clock.Time(), oldClock)
		}
	})
}

func TestSerf_join_pendingIntent(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	upsertIntent(s.recentIntents, "test", messageJoinType, 5, time.Now)
	n := memberlist.Node{Name: "test",
		Addr: nil,
		Meta: []byte("test"),
	}

	s.handleNodeJoin(&n)

	mem := s.members["test"]
	if mem.statusLTime != 5 {
		t.Fatalf("bad join time")
	}
	if mem.Status != StatusAlive {
		t.Fatalf("bad status")
	}
}

func TestSerf_join_pendingIntents(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	upsertIntent(s.recentIntents, "test", messageJoinType, 5, time.Now)
	upsertIntent(s.recentIntents, "test", messageLeaveType, 6, time.Now)
	n := memberlist.Node{Name: "test",
		Addr: nil,
		Meta: []byte("test"),
	}

	s.handleNodeJoin(&n)

	mem := s.members["test"]
	if mem.statusLTime != 6 {
		t.Fatalf("bad join time")
	}
	if mem.Status != StatusLeaving {
		t.Fatalf("bad status")
	}
}

func TestSerf_leaveIntent_bufferEarly(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	// Deliver a leave intent message early
	j := messageLeave{LTime: 10, Node: "test"}
	if !s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we buffered
	if leave, ok := recentIntent(s.recentIntents, "test", messageLeaveType); !ok || leave != 10 {
		t.Fatalf("bad buffer")
	}
}

func TestSerf_leaveIntent_oldMessage(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusAlive,
		},
		statusLTime: 12,
	}

	j := messageLeave{LTime: 10, Node: "test"}
	if s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	if _, ok := recentIntent(s.recentIntents, "test", messageLeaveType); ok {
		t.Fatalf("should not have buffered intent")
	}
}

func TestSerf_leaveIntent_newer(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusAlive,
		},
		statusLTime: 12,
	}

	j := messageLeave{LTime: 14, Node: "test"}
	if !s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if _, ok := recentIntent(s.recentIntents, "test", messageLeaveType); ok {
		t.Fatalf("should not have buffered intent")
	}

	if s.members["test"].Status != StatusLeaving {
		t.Fatalf("should update status")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_joinIntent_bufferEarly(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	// Deliver a join intent message early
	j := messageJoin{LTime: 10, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleNodeJoinIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we buffered
	if join, ok := recentIntent(s.recentIntents, "test", messageJoinType); !ok || join != 10 {
		t.Fatalf("bad buffer")
	}
}

func TestSerf_joinIntent_oldMessage(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		statusLTime: 12,
	}

	j := messageJoin{LTime: 10, Node: "test"}
	if s.handleNodeJoinIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we didn't buffer anything
	if _, ok := recentIntent(s.recentIntents, "test", messageJoinType); ok {
		t.Fatalf("should not have buffered intent")
	}
}

func TestSerf_joinIntent_newer(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		statusLTime: 12,
	}

	// Deliver a join intent message early
	j := messageJoin{LTime: 14, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if _, ok := recentIntent(s.recentIntents, "test", messageJoinType); ok {
		t.Fatalf("should not have buffered intent")
	}

	if s.members["test"].statusLTime != 14 {
		t.Fatalf("should update join time")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_joinIntent_resetLeaving(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusLeaving,
		},
		statusLTime: 12,
	}

	j := messageJoin{LTime: 14, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if _, ok := recentIntent(s.recentIntents, "test", messageJoinType); ok {
		t.Fatalf("should not have buffered intent")
	}

	if s.members["test"].statusLTime != 14 {
		t.Fatalf("should update join time")
	}
	if s.members["test"].Status != StatusAlive {
		t.Fatalf("should update status")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_userEvent_oldMessage(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	// increase the ltime artificially
	s.eventClock.Witness(LamportTime(c.EventBuffer + 1000))

	msg := messageUserEvent{
		LTime:   1,
		Name:    "old",
		Payload: nil,
	}
	if s.handleUserEvent(&msg) {
		t.Fatalf("should not rebroadcast")
	}
}

func TestSerf_userEvent_sameClock(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	eventCh := make(chan Event, 4)
	c := testConfig(t, ip1)
	c.EventCh = eventCh
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	msg := messageUserEvent{
		LTime:   1,
		Name:    "first",
		Payload: []byte("test"),
	}
	if !s.handleUserEvent(&msg) {
		t.Fatalf("should rebroadcast")
	}
	msg = messageUserEvent{
		LTime:   1,
		Name:    "first",
		Payload: []byte("newpayload"),
	}
	if !s.handleUserEvent(&msg) {
		t.Fatalf("should rebroadcast")
	}
	msg = messageUserEvent{
		LTime:   1,
		Name:    "second",
		Payload: []byte("other"),
	}
	if !s.handleUserEvent(&msg) {
		t.Fatalf("should rebroadcast")
	}

	testUserEvents(t, eventCh,
		[]string{"first", "first", "second"},
		[][]byte{[]byte("test"), []byte("newpayload"), []byte("other")})
}

func TestSerf_query_oldMessage(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	// increase the ltime artificially
	s.queryClock.Witness(LamportTime(c.QueryBuffer + 1000))

	msg := messageQuery{
		LTime:   1,
		Name:    "old",
		Payload: nil,
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}
}

func TestSerf_query_sameClock(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	eventCh := make(chan Event, 4)
	c := testConfig(t, ip1)
	c.EventCh = eventCh
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	msg := messageQuery{
		LTime:   1,
		ID:      1,
		Name:    "foo",
		Payload: []byte("test"),
	}
	if !s.handleQuery(&msg) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}
	msg = messageQuery{
		LTime:   1,
		ID:      2,
		Name:    "bar",
		Payload: []byte("newpayload"),
	}
	if !s.handleQuery(&msg) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}
	msg = messageQuery{
		LTime:   1,
		ID:      3,
		Name:    "baz",
		Payload: []byte("other"),
	}
	if !s.handleQuery(&msg) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}

	testQueryEvents(t, eventCh,
		[]string{"foo", "bar", "baz"},
		[][]byte{[]byte("test"), []byte("newpayload"), []byte("other")})
}
