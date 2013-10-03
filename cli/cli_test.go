package cli

import (
	"bytes"
	"strings"
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

func TestCLIRun_printHelp(t *testing.T) {
	testCases := [][]string{
		{},
		{"-h"},
		{"i-dont-exist"},
	}

	for _, testCase := range testCases {
		ui := &MockUi{
			ErrorWriter:  new(bytes.Buffer),
			OutputWriter: new(bytes.Buffer),
		}

		cli := &CLI{
			Args: testCase,
			Ui:   ui,
		}

		code, err := cli.Run()
		if err != nil {
			t.Errorf("Args: %#v. Error: %s", testCase, err)
			continue
		}

		if code != 1 {
			t.Errorf("Args: %#v. Code: %d", testCase, code)
			continue
		}

		if !strings.Contains(ui.ErrorWriter.String(), "usage: ") {
			t.Errorf("Args: %#v. Output: %#v",
				testCase, ui.ErrorWriter.String())
			continue
		}
	}
}

func TestCLISubcommand(t *testing.T) {
	testCases := []struct {
		args       []string
		subcommand string
	}{
		{[]string{"bar"}, "bar"},
		{[]string{"foo", "-h"}, "foo"},
		{[]string{"-h", "bar"}, "bar"},
	}

	for _, testCase := range testCases {
		cli := &CLI{Args: testCase.args}
		result := cli.Subcommand()

		if result != testCase.subcommand {
			t.Errorf("Expected %#v, got %#v. Args: %#v",
				testCase.subcommand, result, testCase.args)
		}
	}
}
