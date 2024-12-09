// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf/internal/race"
	"github.com/hashicorp/serf/testutil"
	"github.com/hashicorp/serf/testutil/retry"
)

func testConfig(t *testing.T, ip net.IP) *Config {
	config := DefaultConfig()
	config.Init()
	config.MemberlistConfig.BindAddr = ip.String()

	// Set probe intervals that are aggressive for finding bad nodes
	config.MemberlistConfig.GossipInterval = 5 * time.Millisecond
	config.MemberlistConfig.ProbeInterval = 50 * time.Millisecond
	config.MemberlistConfig.ProbeTimeout = 25 * time.Millisecond
	config.MemberlistConfig.TCPTimeout = 100 * time.Millisecond
	config.MemberlistConfig.SuspicionMult = 1

	// Activate the strictest version of memberlist validation to ensure
	// we properly pass node names through the serf layer.
	config.MemberlistConfig.RequireNodeNames = true

	config.NodeName = fmt.Sprintf("node-%s", config.MemberlistConfig.BindAddr)

	// Set a short reap interval so that it can run during the test
	config.ReapInterval = 1 * time.Second

	// Set a short reconnect interval so that it can run a lot during tests
	config.ReconnectInterval = 100 * time.Millisecond

	// Set basically zero on the reconnect/tombstone timeouts so that
	// they're removed on the first ReapInterval.
	config.ReconnectTimeout = 1 * time.Microsecond
	config.TombstoneTimeout = 1 * time.Microsecond

	if t != nil {
		config.Logger = log.New(os.Stderr, "test["+t.Name()+"]: ", log.LstdFlags)
		config.MemberlistConfig.Logger = config.Logger
	}

	return config
}

// compatible with testing.TB and *retry.R
type testFailer interface {
	Fatalf(format string, args ...interface{})
}

// testMember tests that a member in a list is in a given state.
func testMember(tf testFailer, members []Member, name string, status MemberStatus) {
	for _, m := range members {
		if m.Name == name {
			if m.Status != status {
				tf.Fatalf("bad state for %s: %d", name, m.Status)
			}
			return
		}
	}

	if status == StatusNone {
		// We didn't expect to find it
		return
	}

	tf.Fatalf("node not found: %s", name)
}

// testMemberStatus is testMember but returns an error
// instead of failing the test
func testMemberStatus(tf testFailer, members []Member, name string, status MemberStatus) {
	for _, m := range members {
		if m.Name == name {
			if m.Status != status {
				tf.Fatalf("bad state for %s: %d", name, m.Status)
			}
			return
		}
	}

	if status == StatusNone {
		// We didn't expect to find it
		return
	}

	tf.Fatalf("node not found: %s", name)
}

func TestCreate_badProtocolVersion(t *testing.T) {
	cases := []struct {
		version uint8
		err     bool
	}{
		{ProtocolVersionMin, false},
		{ProtocolVersionMax, false},
		// TODO(mitchellh): uncommon when we're over 0
		//{ProtocolVersionMin - 1, true},
		{ProtocolVersionMax + 1, true},
		{ProtocolVersionMax - 1, false},
	}

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("version-%d", tc.version), func(t *testing.T) {
			c := testConfig(t, ip1)
			c.ProtocolVersion = tc.version
			s, err := Create(c)
			if tc.err && err == nil {
				t.Errorf("Should've failed with version: %d", tc.version)
			} else if !tc.err && err != nil {
				t.Errorf("Version '%d' error: %s", tc.version, err)
			}

			if err == nil {
				s.Shutdown()
			}
		})
	}
}

func TestSerf_eventsFailed(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh

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

	err = s2.Shutdown()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 1, s1)

	// Since s2 shutdown, we check the events to make sure we got failures.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberReap})
}

func TestSerf_eventsJoin(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh

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

	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin})
}

func TestSerf_eventsLeave(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh
	// Make the reap interval longer in this test
	// so that the leave does not also cause a reap
	s1Config.ReapInterval = 30 * time.Second

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

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		testMemberStatus(r, s1.Members(), s2Config.NodeName, StatusLeft)
	})

	// Now that s2 has left, we check the events to make sure we got
	// a leave event in s1 about the leave.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberLeave})
}

func TestSerf_eventsLeave_avoidInfiniteLeaveRebroadcast(t *testing.T) {
	// This test is a variation of the normal leave test that is crafted
	// specifically to handle a situation where two unique leave events for the
	// same node reach two other nodes in the wrong order which causes them to
	// infinitely rebroadcast the leave event without updating their own
	// lamport clock for that node.
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	ip4, returnFn4 := testutil.TakeIP()
	defer returnFn4()

	testConfigLocal := func(t *testing.T, ip net.IP) *Config {
		conf := testConfig(t, ip)
		// Make the reap interval longer in this test
		// so that the leave does not also cause a reap
		conf.ReapInterval = 30 * time.Second
		return conf
	}

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfigLocal(t, ip1)
	s1Config.EventCh = eventCh

	s2Config := testConfigLocal(t, ip2)
	s2Config.RejoinAfterLeave = true
	s2Addr := s2Config.MemberlistConfig.BindAddr
	s2Name := s2Config.NodeName

	// Allow s3 and s4 to drop joins in the future.
	var dropJoins uint32
	messageDropper := func(t messageType) bool {
		switch t {
		case messageJoinType, messagePushPullType:
			return atomic.LoadUint32(&dropJoins) == 1
		default:
			return false
		}
	}

	s3Config := testConfigLocal(t, ip3)
	s3Config.messageDropper = messageDropper

	s4Config := testConfigLocal(t, ip4)
	s4Config.messageDropper = messageDropper

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

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	s4, err := Create(s4Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s4.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err = s3.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err = s4.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// S2 leaves gracefully
	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make s3 and s4 drop inbound join messages and push-pulls for a bit so it won't see
	// s2 rejoin
	atomic.StoreUint32(&dropJoins, 1)

	// Bring back s2 by mimicking its name and address
	s2Config = testConfigLocal(t, ip2)
	s2Config.RejoinAfterLeave = true
	s2Config.MemberlistConfig.BindAddr = s2Addr
	s2Config.NodeName = s2Name
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	_, err = s2.Join([]string{s1Config.NodeName + "/" + s1Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 4, s1, s2, s3, s4)

	// Now leave a second time but before s3 and s4 see the rejoin (due to the gate)
	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilIntentQueueLen(t, 0, s1, s3, s4)

	retry.Run(t, func(r *retry.R) {
		testMemberStatus(r, s1.Members(), s2Config.NodeName, StatusLeft)
		testMemberStatus(r, s3.Members(), s2Config.NodeName, StatusLeft)
		testMemberStatus(r, s4.Members(), s2Config.NodeName, StatusLeft)
	})

	// Now that s2 has left, we check the events to make sure we got
	// a leave event in s1 about the leave.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberLeave,
			EventMemberJoin, EventMemberLeave})
}

func TestSerf_RemoveFailed_eventsLeave(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh

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

	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	if err := s1.RemoveFailedNode(s2Config.NodeName); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		testMemberStatus(r, s1.Members(), s2Config.NodeName, StatusLeft)
	})

	// Now that s2 has failed and been marked as left, we check the
	// events to make sure we got a leave event in s1 about the leave.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberLeave})
}

func TestSerf_eventsUser(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s2Config.EventCh = eventCh

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

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar"), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// testutil.Yield()

	// check the events to make sure we got
	// a leave event in s1 about the leave.
	testUserEvents(t, eventCh,
		[]string{"event!", "second"},
		[][]byte{[]byte("test"), []byte("foobar")})
}

func TestSerf_eventsUser_sizeLimit(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	// Create the s1 config with an event channel so we can listen
	s1Config := testConfig(t, ip1)
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	waitUntilNumNodes(t, 1, s1)

	name := "this is too large an event"
	payload := make([]byte, s1Config.UserEventSizeLimit)
	err = s1.UserEvent(name, payload, false)
	if err == nil {
		t.Fatalf("expect error")
	}
	if !strings.HasPrefix(err.Error(), "user event exceeds") {
		t.Fatalf("should get size limit error")
	}
}

func TestSerf_getQueueMax(t *testing.T) {
	s := &Serf{
		config: DefaultConfig(),
	}

	// We don't need a running Serf so fake it out with the required
	// state.
	s.members = make(map[string]*memberState)
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("Member%d", i)
		s.members[name] = &memberState{
			Member: Member{
				Name: name,
			},
		}
	}

	// Default mode just uses the max depth.
	if got, want := s.getQueueMax(), 4096; got != want {
		t.Fatalf("got %d want %d", got, want)
	}

	// Now configure a min which should take precedence.
	s.config.MinQueueDepth = 1024
	if got, want := s.getQueueMax(), 1024; got != want {
		t.Fatalf("got %d want %d", got, want)
	}

	// Bring it under the number of nodes, so the calculation based on
	// the number of nodes takes precedence.
	s.config.MinQueueDepth = 16
	if got, want := s.getQueueMax(), 200; got != want {
		t.Fatalf("got %d want %d", got, want)
	}

	// Try adjusting the node count.
	s.members["another"] = &memberState{
		Member: Member{
			Name: "another",
		},
	}
	if got, want := s.getQueueMax(), 202; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestSerf_joinLeave(t *testing.T) {
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

	if err := s1.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Give the reaper time to reap nodes
	time.Sleep(s1Config.ReapInterval * 2)

	waitUntilNumNodes(t, 1, s1, s2)
}

// Bug: GH-58
func TestSerf_leaveRejoinDifferentRole(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s2Config.Tags["role"] = "foo"

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

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.Yield()

	// Make s3 look just like s2, but create a new node with a new role
	s3Config := testConfig(t, ip2)
	s3Config.MemberlistConfig.BindAddr = s2Config.MemberlistConfig.BindAddr
	s3Config.NodeName = s2Config.NodeName
	s3Config.Tags["role"] = "bar"

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	_, err = s3.Join([]string{s1Config.NodeName + "/" + s1Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s3)

	retry.Run(t, func(r *retry.R) {
		var member *Member
		for _, m := range s1.Members() {
			if m.Name == s3Config.NodeName {
				member = &m
				break
			}
		}

		if member == nil {
			r.Fatalf("couldn't find member")
		}

		if member.Tags["role"] != s3Config.Tags["role"] {
			r.Fatalf("bad role: %s", member.Tags["role"])
		}
	})
}

func TestSerf_forceLeaveFailed(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s3Config := testConfig(t, ip3)

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

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2, s3)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 3, s1, s2, s3)

	//Put s2 in failed state
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		testMemberStatus(r, s1.Members(), s2Config.NodeName, StatusFailed)
	})
	if err := s1.forceLeave(s2.config.NodeName, true); err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s3)
}

func TestSerf_forceLeaveLeaving(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s3Config := testConfig(t, ip3)

	//make it so it doesn't get reaped
	// allow for us to see the leaving state
	s1Config.TombstoneTimeout = 1 * time.Hour
	s1Config.LeavePropagateDelay = 5 * time.Second

	s2Config.TombstoneTimeout = 1 * time.Hour
	s2Config.LeavePropagateDelay = 5 * time.Second

	s3Config.TombstoneTimeout = 1 * time.Hour
	s3Config.LeavePropagateDelay = 5 * time.Second

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

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2, s3)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 3, s1, s2, s3)

	//Put s2 in left state
	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		testMemberStatus(r, s1.Members(), s2Config.NodeName, StatusLeft)
	})

	if err := s1.forceLeave(s2.config.NodeName, true); err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s3)
}

func TestSerf_forceLeaveLeft(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s3Config := testConfig(t, ip3)

	//make it so it doesn't get reaped
	s1Config.TombstoneTimeout = 1 * time.Hour
	s2Config.TombstoneTimeout = 1 * time.Hour
	s3Config.TombstoneTimeout = 1 * time.Hour

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

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2, s3)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 3, s1, s2, s3)

	//Put s2 in left state
	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		testMemberStatus(r, s1.Members(), s2Config.NodeName, StatusLeft)
	})

	if err := s1.forceLeave(s2.config.NodeName, true); err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s3)
}

func TestSerf_reconnect(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	eventCh := make(chan Event, 64)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh

	s2Config := testConfig(t, ip2)
	s2Addr := s2Config.MemberlistConfig.BindAddr
	s2Name := s2Config.NodeName

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

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Bring back s2 by mimicking its name and address
	s2Config = testConfig(t, ip2)
	s2Config.MemberlistConfig.BindAddr = s2Addr
	s2Config.NodeName = s2Name
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	waitUntilNumNodes(t, 2, s1, s2)
	// time.Sleep(s1Config.ReconnectInterval * 5)

	testEvents(t, eventCh, s2Name,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberJoin})
}

func TestSerf_reconnect_sameIP(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	eventCh := make(chan Event, 64)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh

	s2Config := testConfig(t, ip2)
	s2Config.MemberlistConfig.BindAddr = s1Config.MemberlistConfig.BindAddr
	s2Config.MemberlistConfig.BindPort = s1Config.MemberlistConfig.BindPort + 1

	s2Addr := fmt.Sprintf("%s:%d",
		s2Config.MemberlistConfig.BindAddr,
		s2Config.MemberlistConfig.BindPort)
	s2Name := s2Config.NodeName

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

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Addr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Bring back s2 by mimicking its name and address
	s2Config = testConfig(t, ip2)
	s2Config.MemberlistConfig.BindAddr = s1Config.MemberlistConfig.BindAddr
	s2Config.MemberlistConfig.BindPort = s1Config.MemberlistConfig.BindPort + 1
	s2Config.NodeName = s2Name
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// time.Sleep(s1Config.ReconnectInterval * 5)
	waitUntilNumNodes(t, 2, s1, s2)

	testEvents(t, eventCh, s2Name,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberJoin})
}

func TestSerf_update(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	eventCh := make(chan Event, 64)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh

	s2Config := testConfig(t, ip2)
	s2Addr := s2Config.MemberlistConfig.BindAddr
	s2Name := s2Config.NodeName

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

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Don't wait for a failure to be detected. Bring back s2 immediately
	// by mimicking its name and address.
	s2Config = testConfig(t, ip2)
	s2Config.MemberlistConfig.BindAddr = s2Addr
	s2Config.NodeName = s2Name

	// Add a tag to force an update event, and add a version downgrade as
	// well (that alone won't trigger an update).
	s2Config.ProtocolVersion--
	s2Config.Tags["foo"] = "bar"

	// We try for a little while to wait for s2 to fully shutdown since the
	// shutdown method doesn't block until that's done.
	start := time.Now()
	for {
		s2, err = Create(s2Config)
		if err == nil {
			defer s2.Shutdown()
			break
		} else if !strings.Contains(err.Error(), "address already in use") {
			t.Fatalf("err: %v", err)
		}

		if time.Now().Sub(start) > 2*time.Second {
			t.Fatalf("timed out trying to restart")
		}
	}

	_, err = s2.Join([]string{s1Config.NodeName + "/" + s1Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)

	testEvents(t, eventCh, s2Name,
		[]EventType{EventMemberJoin, EventMemberUpdate})

	// Verify that the member data got updated.
	found := false
	members := s1.Members()
	for _, member := range members {
		if member.Name == s2Name {
			found = true
			if member.Tags["foo"] != "bar" || member.DelegateCur != s2Config.ProtocolVersion {
				t.Fatalf("bad: %#v", member)
			}
		}
	}
	if !found {
		t.Fatalf("didn't find s2 in members")
	}
}

func TestSerf_role(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)

	s1Config.Tags["role"] = "web"
	s2Config.Tags["role"] = "lb"

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
		roles := make(map[string]string)
		for _, m := range s1.Members() {
			roles[m.Name] = m.Tags["role"]
		}

		if roles[s1Config.NodeName] != "web" {
			r.Fatalf("bad role for web: %s", roles[s1Config.NodeName])
		}

		if roles[s2Config.NodeName] != "lb" {
			r.Fatalf("bad role for lb: %s", roles[s2Config.NodeName])
		}
	})
}

func TestSerfProtocolVersion(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	config := testConfig(t, ip1)
	config.ProtocolVersion = ProtocolVersionMax

	s1, err := Create(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	actual := s1.ProtocolVersion()
	if actual != ProtocolVersionMax {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestSerfRemoveFailedNode(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s3Config := testConfig(t, ip3)

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

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2, s3)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 3, s1, s2, s3)

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	retry.Run(t, func(r *retry.R) {
		// Verify that s2 is "failed"
		testMember(r, s1.Members(), s2Config.NodeName, StatusFailed)
	})

	// Now remove the failed node
	if err := s1.RemoveFailedNode(s2Config.NodeName); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		// Verify that s2 is gone
		testMember(r, s1.Members(), s2Config.NodeName, StatusLeft)
		testMember(r, s3.Members(), s2Config.NodeName, StatusLeft)
	})
}

func TestSerfRemoveFailedNode_prune(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s3Config := testConfig(t, ip3)

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

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2, s3)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 3, s1, s2, s3)

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Verify that s2 is "failed"
	retry.Run(t, func(r *retry.R) {
		testMember(r, s1.Members(), s2Config.NodeName, StatusFailed)
	})

	// Now remove the failed node
	if err := s1.RemoveFailedNodePrune(s2Config.NodeName); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check to make sure it's gone
	waitUntilNumNodes(t, 2, s1, s3)
}

func TestSerfRemoveFailedNode_ourself(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s1Config := testConfig(t, ip1)

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	waitUntilNumNodes(t, 1, s1)

	if err := s1.RemoveFailedNode("somebody"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSerfState(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s1, err := Create(testConfig(t, ip1))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	if s1.State() != SerfAlive {
		t.Fatalf("bad state: %d", s1.State())
	}

	if err := s1.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if s1.State() != SerfLeft {
		t.Fatalf("bad state: %d", s1.State())
	}

	if err := s1.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if s1.State() != SerfShutdown {
		t.Fatalf("bad state: %d", s1.State())
	}
}

func TestSerf_ReapHandler_Shutdown(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s, err := Create(testConfig(t, ip1))
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the reap handler exits on shutdown.
	doneCh := make(chan struct{})
	go func() {
		s.handleReap()
		close(doneCh)
	}()

	s.Shutdown()
	select {
	case <-doneCh:
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestSerf_ReapHandler(t *testing.T) {
	if race.Enabled {
		t.Skip("test contains a data race")
	}

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	c := testConfig(t, ip1)
	c.ReapInterval = time.Nanosecond
	c.TombstoneTimeout = time.Second * 6
	c.RecentIntentTimeout = time.Second * 7
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	m := Member{}
	s.leftMembers = []*memberState{
		{m, 0, time.Now()},
		{m, 0, time.Now().Add(-5 * time.Second)},
		{m, 0, time.Now().Add(-10 * time.Second)},
	}

	upsertIntent(s.recentIntents, "alice", messageJoinType, 1, time.Now)
	upsertIntent(s.recentIntents, "bob", messageJoinType, 2, func() time.Time {
		return time.Now().Add(-10 * time.Second)
	})
	upsertIntent(s.recentIntents, "carol", messageLeaveType, 1, time.Now)
	upsertIntent(s.recentIntents, "doug", messageLeaveType, 2, func() time.Time {
		return time.Now().Add(-10 * time.Second)
	})

	go func() {
		time.Sleep(time.Millisecond)
		s.Shutdown()
	}()

	s.handleReap()

	if len(s.leftMembers) != 2 {
		t.Fatalf("should be shorter")
	}
	if _, ok := recentIntent(s.recentIntents, "alice", messageJoinType); !ok {
		t.Fatalf("should be buffered")
	}
	if _, ok := recentIntent(s.recentIntents, "bob", messageJoinType); ok {
		t.Fatalf("should be reaped")
	}
	if _, ok := recentIntent(s.recentIntents, "carol", messageLeaveType); !ok {
		t.Fatalf("should be buffered")
	}
	if _, ok := recentIntent(s.recentIntents, "doug", messageLeaveType); ok {
		t.Fatalf("should be reaped")
	}
}

func TestSerf_Reap(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s, err := Create(testConfig(t, ip1))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s.Shutdown()

	m := Member{}
	old := []*memberState{
		&memberState{m, 0, time.Now()},
		&memberState{m, 0, time.Now().Add(-5 * time.Second)},
		&memberState{m, 0, time.Now().Add(-10 * time.Second)},
	}

	old = s.reap(old, time.Now(), time.Second*6)
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestRemoveOldMember(t *testing.T) {
	old := []*memberState{
		&memberState{Member: Member{Name: "foo"}},
		&memberState{Member: Member{Name: "bar"}},
		&memberState{Member: Member{Name: "baz"}},
	}

	old = removeOldMember(old, "bar")
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
	if old[1].Name == "bar" {
		t.Fatalf("should remove old member")
	}
}

func TestRecentIntent(t *testing.T) {
	if _, ok := recentIntent(nil, "foo", messageJoinType); ok {
		t.Fatalf("should get nothing on empty recent")
	}

	now := time.Now()
	expire := func() time.Time {
		return now.Add(-2 * time.Second)
	}
	save := func() time.Time {
		return now
	}

	intents := make(map[string]nodeIntent)
	if _, ok := recentIntent(intents, "foo", messageJoinType); ok {
		t.Fatalf("should get nothing on empty recent")
	}
	if added := upsertIntent(intents, "foo", messageJoinType, 1, expire); !added {
		t.Fatalf("should have added")
	}
	if added := upsertIntent(intents, "bar", messageLeaveType, 2, expire); !added {
		t.Fatalf("should have added")
	}
	if added := upsertIntent(intents, "baz", messageJoinType, 3, save); !added {
		t.Fatalf("should have added")
	}
	if added := upsertIntent(intents, "bar", messageJoinType, 4, expire); !added {
		t.Fatalf("should have added")
	}
	if added := upsertIntent(intents, "bar", messageJoinType, 0, expire); added {
		t.Fatalf("should not have added")
	}
	if added := upsertIntent(intents, "bar", messageJoinType, 5, expire); !added {
		t.Fatalf("should have added")
	}

	if ltime, ok := recentIntent(intents, "foo", messageJoinType); !ok || ltime != 1 {
		t.Fatalf("bad: %v %v", ok, ltime)
	}
	if ltime, ok := recentIntent(intents, "bar", messageJoinType); !ok || ltime != 5 {
		t.Fatalf("bad: %v %v", ok, ltime)
	}
	if ltime, ok := recentIntent(intents, "baz", messageJoinType); !ok || ltime != 3 {
		t.Fatalf("bad: %v %v", ok, ltime)
	}
	if _, ok := recentIntent(intents, "tubez", messageJoinType); ok {
		t.Fatalf("should get nothing")
	}

	reapIntents(intents, now, time.Second)
	if _, ok := recentIntent(intents, "foo", messageJoinType); ok {
		t.Fatalf("should get nothing")
	}
	if _, ok := recentIntent(intents, "bar", messageJoinType); ok {
		t.Fatalf("should get nothing")
	}
	if ltime, ok := recentIntent(intents, "baz", messageJoinType); !ok || ltime != 3 {
		t.Fatalf("bad: %v %v", ok, ltime)
	}
	if _, ok := recentIntent(intents, "tubez", messageJoinType); ok {
		t.Fatalf("should get nothing")
	}

	reapIntents(intents, now.Add(2*time.Second), time.Second)
	if _, ok := recentIntent(intents, "baz", messageJoinType); ok {
		t.Fatalf("should get nothing")
	}
}

func TestMemberStatus_String(t *testing.T) {
	status := []MemberStatus{StatusNone, StatusAlive, StatusLeaving, StatusLeft, StatusFailed}
	expect := []string{"none", "alive", "leaving", "left", "failed"}

	for idx, s := range status {
		if s.String() != expect[idx] {
			t.Fatalf("got string %v, expected %v", s.String(), expect[idx])
		}
	}

	other := MemberStatus(100)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	_ = other.String()
}

func TestSerf_joinLeaveJoin(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s1Config.ReapInterval = 10 * time.Second

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	s2Config := testConfig(t, ip2)
	s2Config.ReapInterval = 10 * time.Second

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

	// Leave and shutdown
	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Give the reaper time to reap nodes
	time.Sleep(s1Config.MemberlistConfig.ProbeInterval * 5)

	// s1 should see the node as having left
	retry.Run(t, func(r *retry.R) {
		mems := s1.Members()
		anyLeft := false
		for _, m := range mems {
			if m.Status == StatusLeft {
				anyLeft = true
				break
			}
		}
		if !anyLeft {
			r.Fatalf("node should have left!")
		}
	})

	// Bring node 2 back
	s2Config = testConfig(t, ip2)
	s2Config.ReapInterval = 10 * time.Second
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	waitUntilNumNodes(t, 1, s2)

	// Re-attempt the join
	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		// Should be back to both members
		if s1.NumNodes() != 2 {
			r.Fatalf("s1 members: %d", s1.NumNodes())
		}
		if s2.NumNodes() != 2 {
			r.Fatalf("s2 members: %d", s2.NumNodes())
		}

		// s1 should see the node as alive
		mems := s1.Members()
		anyLeft := false
		for _, m := range mems {
			if m.Status == StatusLeft {
				anyLeft = true
				break
			}
		}
		if anyLeft {
			r.Fatalf("all nodes should be alive!")
		}
	})
}

func TestSerf_Join_IgnoreOld(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s2Config.EventCh = eventCh

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

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar"), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.Yield()

	// join with ignoreOld set to true! should not get events
	_, err = s2.Join([]string{s1Config.NodeName + "/" + s1Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)

	// check the events to make sure we got nothing
	testUserEvents(t, eventCh, []string{}, [][]byte{})
}

func TestSerf_SnapshotRecovery(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s2Config.SnapshotPath = td + "snap"

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

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.Yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 10)

	// Verify that s2 is "failed"
	testMember(t, s1.Members(), s2Config.NodeName, StatusFailed)

	// Now remove the failed node
	if err := s1.RemoveFailedNode(s2Config.NodeName); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that s2 is gone
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)

	// Listen for events
	eventCh := make(chan Event, 4)
	s2Config = testConfig(t, ip2)
	s2Config.SnapshotPath = td + "snap"
	s2Config.EventCh = eventCh

	// Restart s2 from the snapshot now!
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	// Wait for the node to auto rejoin
	start := time.Now()
	for time.Now().Sub(start) < time.Second {
		members := s1.Members()
		if len(members) == 2 && members[0].Status == StatusAlive && members[1].Status == StatusAlive {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify that s2 is "alive"
	testMember(t, s1.Members(), s2Config.NodeName, StatusAlive)
	testMember(t, s2.Members(), s1Config.NodeName, StatusAlive)

	// Check the events to make sure we got nothing
	testUserEvents(t, eventCh, []string{}, [][]byte{})
}

func TestSerf_Leave_SnapshotRecovery(t *testing.T) {
	if race.Enabled {
		t.Skip("test contains a data race")
	}

	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	// Use a longer reap interval to allow the leave intent to propagate before the node is reaped
	s1Config := testConfig(t, ip1)
	s1Config.ReapInterval = 30 * time.Second
	s2Config := testConfig(t, ip2)
	s2Config.SnapshotPath = td + "snap"
	s2Config.ReapInterval = 30 * time.Second

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

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Verify that s2 is "left"
	retry.Run(t, func(r *retry.R) {
		testMember(r, s1.Members(), s2Config.NodeName, StatusLeft)
	})

	// Restart s2 from the snapshot now!
	s2Config.EventCh = nil
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	// Wait for the node to auto rejoin

	// Verify that s2 is didn't join
	retry.Run(t, func(r *retry.R) {
		if s2.NumNodes() != 1 {
			r.Fatalf("bad members: %#v", s2.Members())
		}

		testMember(r, s1.Members(), s2Config.NodeName, StatusLeft)
	})
}

func TestSerf_SetTags(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	s2Config := testConfig(t, ip2)
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

	// Update the tags
	if err := s1.SetTags(map[string]string{"port": "8000"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := s2.SetTags(map[string]string{"datacenter": "east-aws"}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// wait until the tags are updated everywhere before continuing
	retry.Run(t, func(r *retry.R) {
		// Verify the new tags
		m1m := s1.Members()
		m1mTags := make(map[string]map[string]string)
		for _, m := range m1m {
			m1mTags[m.Name] = m.Tags
		}

		if m := m1mTags[s1.config.NodeName]; m["port"] != "8000" {
			r.Fatalf("bad: %v", m1mTags)
		}

		if m := m1mTags[s2.config.NodeName]; m["datacenter"] != "east-aws" {
			r.Fatalf("bad: %v", m1mTags)
		}

		m2m := s2.Members()
		m2mTags := make(map[string]map[string]string)
		for _, m := range m2m {
			m2mTags[m.Name] = m.Tags
		}

		if m := m2mTags[s1.config.NodeName]; m["port"] != "8000" {
			r.Fatalf("bad: %v", m1mTags)
		}

		if m := m2mTags[s2.config.NodeName]; m["datacenter"] != "east-aws" {
			r.Fatalf("bad: %v", m1mTags)
		}
	})

	// we check the events to make sure we got failures.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberUpdate})
}

func TestSerf_Query(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	// Listen for the query
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-eventCh:
				if e.EventType() != EventQuery {
					continue
				}
				q := e.(*Query)
				if err := q.Respond([]byte("test")); err != nil {
					t.Errorf("err: %v", err)
				}
				return
			case <-time.After(time.Second):
				t.Errorf("timeout")
				return
			}
		}
	}()

	s2Config := testConfig(t, ip2)
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

	// Start a query from s2
	params := s2.DefaultQueryParams()
	params.RequestAck = true
	resp, err := s2.Query("load", []byte("sup girl"), params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var acks []string
	var responses []string

	ackCh := resp.AckCh()
	respCh := resp.ResponseCh()
	for i := 0; i < 3; i++ {
		select {
		case a := <-ackCh:
			acks = append(acks, a)

		case r := <-respCh:
			if r.From != s1Config.NodeName {
				t.Fatalf("bad: %v", r)
			}
			if string(r.Payload) != "test" {
				t.Fatalf("bad: %v", r)
			}
			responses = append(responses, r.From)

		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	if len(acks) != 2 {
		t.Fatalf("missing acks: %v", acks)
	}
	if len(responses) != 1 {
		t.Fatalf("missing responses: %v", responses)
	}
}

func TestSerf_Query_Filter(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.EventCh = eventCh
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	// Listen for the query
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-eventCh:
				if e.EventType() != EventQuery {
					continue
				}
				q := e.(*Query)
				if err := q.Respond([]byte("test")); err != nil {
					t.Errorf("err: %v", err)
				}
				return
			case <-time.After(time.Second):
				t.Errorf("timeout")
				return
			}
		}
	}()

	s2Config := testConfig(t, ip2)
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

	s3Config := testConfig(t, ip3)
	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s3)

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 3, s1, s2, s3)

	// Filter to only s1!
	params := s2.DefaultQueryParams()
	params.FilterNodes = []string{s1Config.NodeName}
	params.RequestAck = true
	params.RelayFactor = 1

	// Start a query from s2
	resp, err := s2.Query("load", []byte("sup girl"), params)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var acks []string
	var responses []string

	ackCh := resp.AckCh()
	respCh := resp.ResponseCh()
	for i := 0; i < 2; i++ {
		select {
		case a := <-ackCh:
			acks = append(acks, a)

		case r := <-respCh:
			if r.From != s1Config.NodeName {
				t.Fatalf("bad: %v", r)
			}
			if string(r.Payload) != "test" {
				t.Fatalf("bad: %v", r)
			}
			responses = append(responses, r.From)

		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	if len(acks) != 1 {
		t.Fatalf("missing acks: %v", acks)
	}
	if len(responses) != 1 {
		t.Fatalf("missing responses: %v", responses)
	}
}

func TestSerf_Query_Deduplicate(t *testing.T) {
	s := &Serf{}

	// Set up a dummy query and response
	mq := &messageQuery{
		LTime:   123,
		ID:      123,
		Timeout: time.Second,
		Flags:   queryFlagAck,
	}
	query := newQueryResponse(3, mq)
	response := &messageQueryResponse{
		LTime: mq.LTime,
		ID:    mq.ID,
		From:  "node1",
	}
	s.queryResponse = map[LamportTime]*QueryResponse{mq.LTime: query}

	// Send a few duplicate responses
	s.handleQueryResponse(response)
	s.handleQueryResponse(response)
	response.Flags |= queryFlagAck
	s.handleQueryResponse(response)
	s.handleQueryResponse(response)

	// Ensure we only get one NodeResponse off the channel
	select {
	case <-query.respCh:
	default:
		t.Fatalf("Should have a response")
	}

	select {
	case <-query.ackCh:
	default:
		t.Fatalf("Should have an ack")
	}

	select {
	case <-query.respCh:
		t.Fatalf("Should not have any other responses")
	default:
	}

	select {
	case <-query.ackCh:
		t.Fatalf("Should not have any other acks")
	default:
	}
}

func TestSerf_Query_sizeLimit(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s1Config := testConfig(t, ip1)

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	name := "this is too large a query"
	payload := make([]byte, s1.config.QuerySizeLimit)
	_, err = s1.Query(name, payload, nil)
	if err == nil {
		t.Fatalf("should get error")
	}
	if !strings.HasPrefix(err.Error(), "query exceeds limit of ") {
		t.Fatalf("should get size limit error: %v", err)
	}
}

func TestSerf_Query_sizeLimitIncreased(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s1Config := testConfig(t, ip1)

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	name := "this is too large a query"
	payload := make([]byte, s1.config.QuerySizeLimit)
	s1.config.QuerySizeLimit = 2 * s1.config.QuerySizeLimit
	_, err = s1.Query(name, payload, nil)
	if err != nil {
		t.Fatalf("should not get error: %v", err)
	}
}

func TestSerf_NameResolution(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s2Config := testConfig(t, ip2)
	s3Config := testConfig(t, ip3)

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

	// Create an artificial node name conflict!
	s3Config.NodeName = s1Config.NodeName
	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2, s3)

	// Join s1 to s2 first. s2 should vote for s1 in conflict
	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)
	waitUntilNumNodes(t, 1, s3)

	_, err = s1.Join([]string{s3Config.NodeName + "/" + s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the query period to end
	time.Sleep(s1.DefaultQueryTimeout() * 2)

	retry.Run(t, func(r *retry.R) {
		// s3 should have shutdown, while s1 is running
		if s1.State() != SerfAlive {
			r.Fatalf("bad: %v", s1.State())
		}
		if s2.State() != SerfAlive {
			r.Fatalf("bad: %v", s2.State())
		}
		if s3.State() != SerfShutdown {
			r.Fatalf("bad: %v", s3.State())
		}
	})
}

func TestSerf_LocalMember(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s1Config := testConfig(t, ip1)

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	m := s1.LocalMember()
	if m.Name != s1Config.NodeName {
		t.Fatalf("bad: %v", m)
	}
	if !reflect.DeepEqual(m.Tags, s1Config.Tags) {
		t.Fatalf("bad: %v", m)
	}
	if m.Status != StatusAlive {
		t.Fatalf("bad: %v", m)
	}

	newTags := map[string]string{
		"foo":  "bar",
		"test": "ing",
	}
	if err := s1.SetTags(newTags); err != nil {
		t.Fatalf("err: %v", err)
	}

	m = s1.LocalMember()
	if !reflect.DeepEqual(m.Tags, newTags) {
		t.Fatalf("bad: %v", m)
	}
}

func TestSerf_WriteKeyringFile(t *testing.T) {
	existing := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="
	newKey := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="

	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	keyringFile := filepath.Join(td, "tags.json")

	existingBytes, err := base64.StdEncoding.DecodeString(existing)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	keys := [][]byte{existingBytes}
	keyring, err := memberlist.NewKeyring(keys, existingBytes)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	s1Config := testConfig(t, ip1)
	s1Config.MemberlistConfig.Keyring = keyring
	s1Config.KeyringFile = keyringFile
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	manager := s1.KeyManager()

	if _, err := manager.InstallKey(newKey); err != nil {
		t.Fatalf("err: %v", err)
	}

	content, err := ioutil.ReadFile(keyringFile)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) != 4 {
		t.Fatalf("bad: %v", lines)
	}

	// Ensure both the original key and the new key are present in the file
	if !strings.Contains(string(content), existing) {
		t.Fatalf("key not found in keyring file: %s", existing)
	}
	if !strings.Contains(string(content), newKey) {
		t.Fatalf("key not found in keyring file: %s", newKey)
	}

	// Ensure the existing key remains primary. This is in position 1 because
	// the file writer will use json.MarshalIndent(), leaving the first line as
	// the opening bracket.
	if !strings.Contains(lines[1], existing) {
		t.Fatalf("expected key to be primary: %s", existing)
	}

	// Swap primary keys
	if _, err := manager.UseKey(newKey); err != nil {
		t.Fatalf("err: %v", err)
	}

	content, err = ioutil.ReadFile(keyringFile)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	lines = strings.Split(string(content), "\n")
	if len(lines) != 4 {
		t.Fatalf("bad: %v", lines)
	}

	// Key order should have changed in keyring file
	if !strings.Contains(lines[1], newKey) {
		t.Fatalf("expected key to be primary: %s", newKey)
	}

	// Remove the old key
	if _, err := manager.RemoveKey(existing); err != nil {
		t.Fatalf("err: %v", err)
	}

	content, err = ioutil.ReadFile(keyringFile)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	lines = strings.Split(string(content), "\n")
	if len(lines) != 3 {
		t.Fatalf("bad: %v", lines)
	}

	// Only the new key should now be present in the keyring file
	if len(lines) != 3 {
		t.Fatalf("bad: %v", lines)
	}
	if !strings.Contains(lines[1], newKey) {
		t.Fatalf("expected key to be primary: %s", newKey)
	}
}

func TestSerfStats(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	config := testConfig(t, ip1)
	s1, err := Create(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	stats := s1.Stats()

	expected := map[string]string{
		"event_queue":  "0",
		"event_time":   "1",
		"failed":       "0",
		"intent_queue": "0",
		"left":         "0",
		"health_score": "0",
		"member_time":  "1",
		"members":      "1",
		"query_queue":  "0",
		"query_time":   "1",
		"encrypted":    "false",
	}

	for key, val := range expected {
		v, ok := stats[key]
		if !ok {
			t.Fatalf("key not found in stats: %s", key)
		}
		if v != val {
			t.Fatalf("bad: %s = %s", key, val)
		}
	}
}

type CancelMergeDelegate struct {
	invoked bool
}

func (c *CancelMergeDelegate) NotifyMerge(members []*Member) error {
	c.invoked = true
	return fmt.Errorf("Merge canceled")
}

func TestSerf_Join_Cancel(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	merge1 := &CancelMergeDelegate{}
	s1Config.Merge = merge1

	s2Config := testConfig(t, ip2)
	merge2 := &CancelMergeDelegate{}
	s2Config.Merge = merge2

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

	waitUntilNumNodes(t, 0, s1, s2)

	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err == nil {
		t.Fatalf("expect error")
	}
	if !strings.Contains(err.Error(), "Merge canceled") {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 0, s1, s2)

	if !merge1.invoked {
		t.Fatalf("should invoke")
	}
	if !merge2.invoked {
		t.Fatalf("should invoke")
	}
}

func TestSerf_Coordinates(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	ip3, returnFn3 := testutil.TakeIP()
	defer returnFn3()

	s1Config := testConfig(t, ip1)
	s1Config.DisableCoordinates = false
	s1Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s1.Shutdown()

	s2Config := testConfig(t, ip2)
	s2Config.DisableCoordinates = false
	s2Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond
	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s2.Shutdown()

	waitUntilNumNodes(t, 1, s1, s2)

	// Make sure both nodes start out the origin so we can prove they did
	// an update later.
	c1, err := s1.GetCoordinate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	c2, err := s2.GetCoordinate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	const zeroThreshold = 20.0e-6
	if c1.DistanceTo(c2).Seconds() > zeroThreshold {
		t.Fatalf("coordinates didn't start at the origin")
	}

	// Join the two nodes together and give them time to probe each other.
	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("could not join s1 and s2: %s", err)
	}

	waitUntilNumNodes(t, 2, s1, s2)
	retry.Run(t, func(r *retry.R) {
		// See if they know about each other.

		if _, ok := s1.GetCachedCoordinate(s2.config.NodeName); !ok {
			r.Fatalf("s1 didn't get a coordinate for s2: %s", err)
		}
		if _, ok := s2.GetCachedCoordinate(s1.config.NodeName); !ok {
			r.Fatalf("s2 didn't get a coordinate for s1: %s", err)
		}

		// With only one ping they won't have a good estimate of the other node's
		// coordinate, but they should both have updated their own coordinate.
		c1, err = s1.GetCoordinate()
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		c2, err = s2.GetCoordinate()
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		if c1.DistanceTo(c2).Seconds() < zeroThreshold {
			r.Fatalf("coordinates didn't update after probes")
		}

		// Make sure they cached their own current coordinate after the update.
		c1c, ok := s1.GetCachedCoordinate(s1.config.NodeName)
		if !ok {
			r.Fatalf("s1 didn't cache coordinate for s1")
		}
		if !reflect.DeepEqual(c1, c1c) {
			r.Fatalf("coordinates are not equal: %v != %v", c1, c1c)
		}
	})

	// Break up the cluster and make sure the coordinates get removed by
	// the reaper.
	if err := s2.Leave(); err != nil {
		t.Fatalf("s2 could not leave: %s", err)
	}

	time.Sleep(s1Config.ReapInterval * 2)

	waitUntilNumNodes(t, 1, s1)
	retry.Run(t, func(r *retry.R) {
		if _, ok := s1.GetCachedCoordinate(s2.config.NodeName); ok {
			r.Fatalf("s1 should have removed s2's cached coordinate")
		}
	})

	// Try a setup with coordinates disabled.
	s3Config := testConfig(t, ip3)
	s3Config.DisableCoordinates = true
	s3Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond
	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer s3.Shutdown()

	waitUntilNumNodes(t, 1, s1, s3)

	_, err = s3.Join([]string{s1Config.NodeName + "/" + s1Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("could not join s1 and s3: %s", err)
	}

	waitUntilNumNodes(t, 2, s1, s3)
	retry.Run(t, func(r *retry.R) {
		_, err = s3.GetCoordinate()
		if err == nil || !strings.Contains(err.Error(), "Coordinates are disabled") {
			r.Fatalf("expected coordinate disabled error, got %s", err)
		}
		if _, ok := s3.GetCachedCoordinate(s1.config.NodeName); ok {
			r.Fatalf("should not have been able to get cached coordinate")
		}
	})
}

// pingVersionMetaDelegate is used to monkey patch a ping delegate so that it
// sends ping messages with an unknown version number.
type pingVersionMetaDelegate struct {
	pingDelegate
}

// AckPayload is called to produce a payload to send back in response to a ping
// request. In this case we send back a bogus ping response with a bad version
// and payload.
func (p *pingVersionMetaDelegate) AckPayload() []byte {
	var buf bytes.Buffer

	// Send back the next ping version, which is bad by default.
	version := []byte{PingVersion + 1}
	buf.Write(version)

	buf.Write([]byte("this is bad and not a real message"))
	return buf.Bytes()
}

func TestSerf_PingDelegateVersioning(t *testing.T) {
	if race.Enabled {
		t.Skip("test contains a data race")
	}

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s1Config.DisableCoordinates = false
	s1Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond
	s2Config := testConfig(t, ip2)
	s2Config.DisableCoordinates = false
	s2Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond

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

	// Monkey patch s1 to send weird versions of the ping messages.
	s1.config.MemberlistConfig.Ping = &pingVersionMetaDelegate{pingDelegate{s1}}

	waitUntilNumNodes(t, 1, s1, s2)

	// Join the two nodes together and give them time to probe each other.
	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("could not join s1 and s2: %s", err)
	}

	// They both should show 2 members, but only s1 should know about s2
	// in the cache, since s1 spoke an alien ping protocol.
	waitUntilNumNodes(t, 2, s1, s2)
	retry.Run(t, func(r *retry.R) {
		if _, ok := s1.GetCachedCoordinate(s2.config.NodeName); !ok {
			r.Fatalf("s1 didn't get a coordinate for s2: %s", err)
		}
		if _, ok := s2.GetCachedCoordinate(s1.config.NodeName); ok {
			r.Fatalf("s2 got an unexpected coordinate for s1")
		}
	})
}

// pingDimensionMetaDelegate is used to monkey patch a ping delegate so that it
// sends coordinates with the wrong number of dimensions.
type pingDimensionMetaDelegate struct {
	t *testing.T
	pingDelegate
}

// AckPayload is called to produce a payload to send back in response to a ping
// request. In this case we send back a legit ping response with a bad coordinate.
func (p *pingDimensionMetaDelegate) AckPayload() []byte {
	var buf bytes.Buffer

	// The first byte is the version number, forming a simple header.
	version := []byte{PingVersion}
	buf.Write(version)

	// Make a bad coordinate with the wrong number of dimensions.
	coord := coordinate.NewCoordinate(coordinate.DefaultConfig())
	coord.Vec = make([]float64, 2*len(coord.Vec))

	// The rest of the message is the serialized coordinate.
	enc := codec.NewEncoder(&buf, &codec.MsgpackHandle{})
	if err := enc.Encode(coord); err != nil {
		p.t.Fatalf("err: %v", err)
	}
	return buf.Bytes()
}

func TestSerf_PingDelegateRogueCoordinate(t *testing.T) {
	if race.Enabled {
		t.Skip("test contains a data race")
	}

	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	s1Config := testConfig(t, ip1)
	s1Config.DisableCoordinates = false
	s1Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond
	s2Config := testConfig(t, ip2)
	s2Config.DisableCoordinates = false
	s2Config.MemberlistConfig.ProbeInterval = time.Duration(2) * time.Millisecond

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

	// Monkey patch s1 to send ping messages with bad coordinates.
	s1.config.MemberlistConfig.Ping = &pingDimensionMetaDelegate{t, pingDelegate{s1}}

	waitUntilNumNodes(t, 1, s1, s2)

	// Join the two nodes together and give them time to probe each other.
	_, err = s1.Join([]string{s2Config.NodeName + "/" + s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("could not join s1 and s2: %s", err)
	}

	// They both should show 2 members, but only s1 should know about s2
	// in the cache, since s1 sent a bad coordinate.
	waitUntilNumNodes(t, 2, s1, s2)
	retry.Run(t, func(r *retry.R) {
		if _, ok := s1.GetCachedCoordinate(s2.config.NodeName); !ok {
			r.Fatalf("s1 didn't get a coordinate for s2: %s", err)
		}
		if _, ok := s2.GetCachedCoordinate(s1.config.NodeName); ok {
			r.Fatalf("s2 got an unexpected coordinate for s1")
		}
	})
}

func TestSerf_NumNodes(t *testing.T) {
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

	if s1.NumNodes() != 1 {
		t.Fatalf("Expected 1 members")
	}

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
}

func waitUntilNumNodes(t *testing.T, desiredNodes int, serfs ...*Serf) {
	t.Helper()
	retry.Run(t, func(r *retry.R) {
		t.Helper()
		for i, s := range serfs {
			if n := s.NumNodes(); desiredNodes != n {
				r.Fatalf("s%d got %d expected %d", i+1, n, desiredNodes)
			}
		}
	})
}

func waitUntilIntentQueueLen(t *testing.T, desiredLen int, serfs ...*Serf) {
	t.Helper()
	retry.Run(t, func(r *retry.R) {
		t.Helper()
		for i, s := range serfs {
			stats := s.Stats()
			iq, err := strconv.Atoi(stats["intent_queue"])
			if err != nil {
				r.Fatalf("err: %v", err)
			}

			if desiredLen != iq {
				r.Fatalf("s%d got %d expected %d", (i + 1), iq, desiredLen)
			}
		}
	})
}

func TestSerf_ValidateNodeName(t *testing.T) {
	type test struct {
		nodename string
		want     string
	}

	tests := []test{
		{
			nodename: "!BadChars&^*",
			want:     "invalid characters",
		},
		{
			nodename: "thisisonehundredandtwentyeightcharacterslongnodenametestaswehavetotesteachcaseeventheoonesonehundredandtwentyeightcharacterslong1",
			want:     "Valid length",
		},
	}
	for _, tc := range tests {
		agentConfig := DefaultConfig()
		agentConfig.NodeName = tc.nodename
		agentConfig.ValidateNodeNames = true
		_, err := Create(agentConfig)
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected: %v, got: %v", tc.want, err)
		}
	}

}

type reconnectOverride struct {
	timeout time.Duration
	called  bool
}

func (r *reconnectOverride) ReconnectTimeout(_ *Member, _ time.Duration) time.Duration {
	r.called = true
	return r.timeout
}

func TestSerf_perNodeReconnectTimeout(t *testing.T) {
	ip1, returnFn1 := testutil.TakeIP()
	defer returnFn1()

	ip2, returnFn2 := testutil.TakeIP()
	defer returnFn2()

	override := reconnectOverride{timeout: 1 * time.Microsecond}

	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig(t, ip1)
	s1Config.ReconnectTimeout = 30 * time.Second
	s1Config.ReconnectTimeoutOverride = &override
	s1Config.EventCh = eventCh

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

	err = s2.Shutdown()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	waitUntilNumNodes(t, 1, s1)

	// Since s2 shutdown, we check the events to make sure we got failures.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberReap})

	if !override.called {
		t.Fatalf("The reconnect override was not used")
	}
}
