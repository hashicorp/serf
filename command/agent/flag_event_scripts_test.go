package agent

import (
	"flag"
	"github.com/hashicorp/serf/serf"
	"testing"
)

func TestEventScriptInvoke(t *testing.T) {
	testCases := []struct {
		script EventScript
		event  serf.Event
		invoke bool
	}{
		{
			EventScript{"*", "", "script.sh"},
			serf.MemberEvent{},
			true,
		},
		{
			EventScript{"user", "", "script.sh"},
			serf.MemberEvent{},
			false,
		},
		{
			EventScript{"user", "deploy", "script.sh"},
			serf.UserEvent{Name: "deploy"},
			true,
		},
		{
			EventScript{"user", "deploy", "script.sh"},
			serf.UserEvent{Name: "restart"},
			false,
		},
		{
			EventScript{"member-join", "", "script.sh"},
			serf.MemberEvent{Type: serf.EventMemberJoin},
			true,
		},
		{
			EventScript{"member-join", "", "script.sh"},
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
	}

	for _, tc := range testCases {
		script := EventScript{Event: tc.Event}
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
			[]EventScript{{"*", "", "script.sh"}},
		},

		{
			"member-join=script.sh",
			false,
			[]EventScript{{"member-join", "", "script.sh"}},
		},

		{
			"foo,bar=script.sh",
			false,
			[]EventScript{
				{"foo", "", "script.sh"},
				{"bar", "", "script.sh"},
			},
		},

		{
			"user:deploy=script.sh",
			false,
			[]EventScript{{"user", "deploy", "script.sh"}},
		},

		{
			"foo,user:blah,bar=script.sh",
			false,
			[]EventScript{
				{"foo", "", "script.sh"},
				{"user", "blah", "script.sh"},
				{"bar", "", "script.sh"},
			},
		},
	}

	for _, tc := range testCases {
		results, err := ParseEventScript(tc.v)
		if tc.err && err == nil {
			t.Errorf("should error: %s", tc.v)
			continue
		} else if !tc.err && err != nil {
			t.Errorf("should not err: %s, %s", tc.v, err)
			continue
		}

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

func TestFlagEventScripts_impl(t *testing.T) {
	var result []EventScript
	var _ flag.Value = (*FlagEventScripts)(&result)
}

func TestFlagEventScripts(t *testing.T) {
	var result []EventScript

	args := []string{
		"-event-script=foo=bar.sh",
		"-event-script=user:deploy=deploy.sh",
	}

	f := flag.NewFlagSet("test", flag.ContinueOnError)
	f.Var((*FlagEventScripts)(&result), "event-script", "foo")
	if err := f.Parse(args); err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := []EventScript{
		{"foo", "", "bar.sh"},
		{"user", "deploy", "deploy.sh"},
	}

	if len(result) != len(expected) {
		t.Fatalf("bad: %#v", result)
	}

	for i, actual := range result {
		if actual != expected[i] {
			t.Fatalf("bad: %#v", actual)
		}
	}
}
