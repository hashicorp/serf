package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"io/ioutil"
	"net"
	"testing"
)

const eventScript = `#!/bin/sh
RESULT_FILE="%s"
echo $SERF_SELF_NAME $SERF_SELF_ROLE >>${RESULT_FILE}
echo $SERF_TAG_DC >> ${RESULT_FILE}
echo $SERF_EVENT $SERF_USER_EVENT "$@" >>${RESULT_FILE}
while read line; do
	printf "${line}\n" >>${RESULT_FILE}
done
`

const userEventScript = `#!/bin/sh
RESULT_FILE="%s"
echo $SERF_SELF_NAME $SERF_SELF_ROLE >>${RESULT_FILE}
echo $SERF_TAG_DC >> ${RESULT_FILE}
echo $SERF_EVENT $SERF_USER_EVENT "$@" >>${RESULT_FILE}
echo $SERF_EVENT $SERF_USER_LTIME "$@" >>${RESULT_FILE}
while read line; do
	printf "${line}\n" >>${RESULT_FILE}
done
`

// testEventScript creates an event script that can be used with the
// agent. It returns the path to the event script itself and a path to
// the file that will contain the events that that script receives.
func testEventScript(t *testing.T, script string) (string, string) {
	scriptFile, err := ioutil.TempFile("", "serf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer scriptFile.Close()

	if err := scriptFile.Chmod(0755); err != nil {
		t.Fatalf("err: %s", err)
	}

	resultFile, err := ioutil.TempFile("", "serf-result")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer resultFile.Close()

	_, err = scriptFile.Write([]byte(
		fmt.Sprintf(script, resultFile.Name())))
	if err != nil {
		t.Fatalf("err: %s")
	}

	return scriptFile.Name(), resultFile.Name()
}

func TestScriptEventHandler(t *testing.T) {
	script, results := testEventScript(t, eventScript)

	h := &ScriptEventHandler{
		Self: serf.Member{
			Name: "ourname",
			Tags: map[string]string{"role": "ourrole", "dc": "east-aws"},
		},
		Scripts: []EventScript{
			{
				EventFilter: EventFilter{
					Event: "*",
				},
				Script: script,
			},
		},
	}

	event := serf.MemberEvent{
		Type: serf.EventMemberJoin,
		Members: []serf.Member{
			{
				Name: "foo",
				Addr: net.ParseIP("1.2.3.4"),
				Tags: map[string]string{"role": "bar", "foo": "bar"},
			},
		},
	}

	h.HandleEvent(event)

	result, err := ioutil.ReadFile(results)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := "ourname ourrole\neast-aws\nmember-join\nfoo\t1.2.3.4\tbar\trole=bar,foo=bar\n"
	if string(result) != expected {
		t.Fatalf("bad: %#v. Expected: %#v", string(result), expected)
	}
}

func TestScriptUserEventHandler(t *testing.T) {
	script, results := testEventScript(t, userEventScript)

	h := &ScriptEventHandler{
		Self: serf.Member{
			Name: "ourname",
			Tags: map[string]string{"role": "ourrole", "dc": "east-aws"},
		},
		Scripts: []EventScript{
			{
				EventFilter: EventFilter{
					Event: "*",
				},
				Script: script,
			},
		},
	}

	userEvent := serf.UserEvent{
		LTime:    1,
		Name:     "baz",
		Payload:  []byte("foobar"),
		Coalesce: true,
	}

	h.HandleEvent(userEvent)

	result, err := ioutil.ReadFile(results)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := "ourname ourrole\neast-aws\nuser baz\nuser 1\n"
	if string(result) != expected {
		t.Fatalf("bad: %#v. Expected: %#v", string(result), expected)
	}
}

func TestEventScriptInvoke(t *testing.T) {
	testCases := []struct {
		script EventScript
		event  serf.Event
		invoke bool
	}{
		{
			EventScript{EventFilter{"*", ""}, "script.sh"},
			serf.MemberEvent{},
			true,
		},
		{
			EventScript{EventFilter{"user", ""}, "script.sh"},
			serf.MemberEvent{},
			false,
		},
		{
			EventScript{EventFilter{"user", "deploy"}, "script.sh"},
			serf.UserEvent{Name: "deploy"},
			true,
		},
		{
			EventScript{EventFilter{"user", "deploy"}, "script.sh"},
			serf.UserEvent{Name: "restart"},
			false,
		},
		{
			EventScript{EventFilter{"member-join", ""}, "script.sh"},
			serf.MemberEvent{Type: serf.EventMemberJoin},
			true,
		},
		{
			EventScript{EventFilter{"member-join", ""}, "script.sh"},
			serf.MemberEvent{Type: serf.EventMemberLeave},
			false,
		},
	}

	for _, tc := range testCases {
		result := tc.script.Invoke(tc.event)
		if result != tc.invoke {
			t.Errorf("bad: %#v", tc)
		}
	}
}

func TestEventScriptValid(t *testing.T) {
	testCases := []struct {
		Event string
		Valid bool
	}{
		{"member-join", true},
		{"member-leave", true},
		{"member-failed", true},
		{"user", true},
		{"User", false},
		{"member", false},
		{"*", true},
	}

	for _, tc := range testCases {
		script := EventScript{EventFilter: EventFilter{Event: tc.Event}}
		if script.Valid() != tc.Valid {
			t.Errorf("bad: %#v", tc)
		}
	}
}

func TestParseEventScript(t *testing.T) {
	testCases := []struct {
		v       string
		err     bool
		results []EventScript
	}{
		{
			"script.sh",
			false,
			[]EventScript{{EventFilter{"*", ""}, "script.sh"}},
		},

		{
			"member-join=script.sh",
			false,
			[]EventScript{{EventFilter{"member-join", ""}, "script.sh"}},
		},

		{
			"foo,bar=script.sh",
			false,
			[]EventScript{
				{EventFilter{"foo", ""}, "script.sh"},
				{EventFilter{"bar", ""}, "script.sh"},
			},
		},

		{
			"user:deploy=script.sh",
			false,
			[]EventScript{{EventFilter{"user", "deploy"}, "script.sh"}},
		},

		{
			"foo,user:blah,bar=script.sh",
			false,
			[]EventScript{
				{EventFilter{"foo", ""}, "script.sh"},
				{EventFilter{"user", "blah"}, "script.sh"},
				{EventFilter{"bar", ""}, "script.sh"},
			},
		},
	}

	for _, tc := range testCases {
		results := ParseEventScript(tc.v)
		if results == nil {
			t.Errorf("result should not be nil")
			continue
		}

		if len(results) != len(tc.results) {
			t.Errorf("bad: %#v", results)
			continue
		}

		for i, r := range results {
			expected := tc.results[i]

			if r.Event != expected.Event {
				t.Errorf("Events not equal: %s %s", r.Event, expected.Event)
			}

			if r.UserEvent != expected.UserEvent {
				t.Errorf("User events not equal: %s %s", r.UserEvent, expected.UserEvent)
			}

			if r.Script != expected.Script {
				t.Errorf("Scripts not equal: %s %s", r.Script, expected.Script)
			}
		}
	}
}

func TestParseEventFilter(t *testing.T) {
	testCases := []struct {
		v       string
		results []EventFilter
	}{
		{
			"",
			[]EventFilter{EventFilter{"*", ""}},
		},

		{
			"member-join",
			[]EventFilter{EventFilter{"member-join", ""}},
		},

		{
			"foo,bar",
			[]EventFilter{
				EventFilter{"foo", ""},
				EventFilter{"bar", ""},
			},
		},

		{
			"user:deploy",
			[]EventFilter{EventFilter{"user", "deploy"}},
		},

		{
			"foo,user:blah,bar",
			[]EventFilter{
				EventFilter{"foo", ""},
				EventFilter{"user", "blah"},
				EventFilter{"bar", ""},
			},
		},
	}

	for _, tc := range testCases {
		results := ParseEventFilter(tc.v)
		if results == nil {
			t.Errorf("result should not be nil")
			continue
		}

		if len(results) != len(tc.results) {
			t.Errorf("bad: %#v", results)
			continue
		}

		for i, r := range results {
			expected := tc.results[i]

			if r.Event != expected.Event {
				t.Errorf("Events not equal: %s %s", r.Event, expected.Event)
			}

			if r.UserEvent != expected.UserEvent {
				t.Errorf("User events not equal: %s %s", r.UserEvent, expected.UserEvent)
			}
		}
	}
}
