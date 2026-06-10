// Copyright IBM Corp. 2013, 2026
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"io"
	"slices"

	"github.com/hashicorp/logutils"
)

// LevelFilter returns a LevelFilter that is configured with the log
// levels that we use.
func LevelFilter() *logutils.LevelFilter {
	return &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERR"},
		MinLevel: "INFO",
		Writer:   io.Discard,
	}
}

// ValidateLevelFilter verifies that the log levels within the filter
// are valid.
func ValidateLevelFilter(minLevel logutils.LogLevel, filter *logutils.LevelFilter) bool {
	return slices.Contains(filter.Levels, minLevel)
}
