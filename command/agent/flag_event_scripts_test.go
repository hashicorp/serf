package agent

import (
	"flag"
	"testing"
)

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
