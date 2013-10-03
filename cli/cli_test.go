package cli

import (
	"testing"
)

func TestCLIIsHelp(t *testing.T) {
	testCases := []struct {
		args   []string
		isHelp bool
	}{
		{[]string{"foo", "-h"}, true},
		{[]string{"foo", "--help"}, true},
		{[]string{"foo", "-h", "bar"}, true},
		{[]string{"foo", "bar"}, false},
		{[]string{"-h", "bar"}, true},
	}

	for _, testCase := range testCases {
		cli := &CLI{Args: testCase.args}
		result := cli.IsHelp()

		if result != testCase.isHelp {
			t.Errorf("Expected '%#v'. Args: %#v", testCase.isHelp, testCase.args)
		}
	}
}
