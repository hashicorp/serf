package agent

import (
	"reflect"
	"testing"
)

func TestConfigEventScripts(t *testing.T) {
	c := &Config{
		EventHandlers: []string{
			"foo.sh",
			"bar=blah.sh",
		},
	}

	result, err := c.EventScripts()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) != 2 {
		t.Fatalf("bad: %#v", result)
	}

	expected := []EventScript{
		{"*", "", "foo.sh"},
		{"bar", "", "blah.sh"},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("bad: %#v", result)
	}
}
