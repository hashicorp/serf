// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"encoding/base64"
	"testing"

	"github.com/mitchellh/cli"
)

func TestKeygenCommand(t *testing.T) {
	ui := new(cli.MockUi)
	c := &KeygenCommand{Ui: ui}
	code := c.Run(nil)
	if code != 0 {
		t.Fatalf("bad: %d", code)
	}

	output := ui.OutputWriter.String()
	result, err := base64.StdEncoding.DecodeString(output)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(result) != 32 {
		t.Fatalf("bad: %#v", result)
	}
}
