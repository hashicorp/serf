// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"io"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestLogger(t testing.TB) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Output: &testWriter{t},
		Name:   "test: ",
	})
}

func TestLoggerWithName(t testing.TB, name string) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Output: &testWriter{t},
		Name:   "test[" + name + "]: ",
	})
}

func TestWriter(t testing.TB) io.Writer {
	return &testWriter{t}
}

type testWriter struct {
	t testing.TB
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.t.Helper()
	tw.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}
