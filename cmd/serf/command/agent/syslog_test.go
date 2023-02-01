// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"runtime"
	"testing"

	"github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
)

func TestSyslogFilter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	l, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, "LOCAL0", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")

	s := &SyslogWrapper{l, filt}
	n, err := s.Write([]byte("[INFO] test"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n == 0 {
		t.Fatalf("should have logged")
	}

	n, err = s.Write([]byte("[DEBUG] test"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 0 {
		t.Fatalf("should not have logged")
	}
}
