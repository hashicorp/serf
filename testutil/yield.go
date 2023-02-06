// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"time"
)

func Yield() {
	time.Sleep(10 * time.Millisecond)
}
