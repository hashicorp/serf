package agent

import (
	"reflect"
	"testing"
)

func TestConfigBindAddrParts(t *testing.T) {
	testCases := []struct {
		Value string
		IP    string
		Port  int
		Error bool
	}{
		{"0.0.0.0", "0.0.0.0", DefaultBindPort, false},
		{"0.0.0.0:1234", "0.0.0.0", 1234, false},
	}

	for _, tc := range testCases {
		c := &Config{BindAddr: tc.Value}
		ip, port, err := c.BindAddrParts()
		if tc.Error != (err != nil) {
			t.Errorf("Bad error: %s", err)
			continue
		}

		if tc.IP != ip {
			t.Errorf("%s: Got IP %#v", tc.Value, ip)
			continue
		}

		if tc.Port != port {
			t.Errorf("%s: Got port %d", tc.Value, port)
			continue
		}
	}
}

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
