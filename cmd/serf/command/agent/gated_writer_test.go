// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"testing"
)

func TestGatedWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	w := &GatedWriter{Writer: buf}
	w.Write([]byte("foo\n"))
	w.Write([]byte("bar\n"))

	if buf.String() != "" {
		t.Fatalf("bad: %s", buf.String())
	}

	w.Flush()

	if buf.String() != "foo\nbar\n" {
		t.Fatalf("bad: %s", buf.String())
	}

	w.Write([]byte("baz\n"))

	if buf.String() != "foo\nbar\nbaz\n" {
		t.Fatalf("bad: %s", buf.String())
	}
}
