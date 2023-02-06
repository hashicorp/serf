// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"reflect"
	"testing"
)

func TestAppendSliceValueSet(t *testing.T) {
	sv := new(AppendSliceValue)
	err := sv.Set("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = sv.Set("bar")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expected := []string{"foo", "bar"}
	if !reflect.DeepEqual([]string(*sv), expected) {
		t.Fatalf("Bad: %#v", sv)
	}
}
