package agent

import (
	"github.com/hashicorp/logutils"
	"io/ioutil"
)

// levelFilter returns a LevelFilter that is configured with the log
// levels that we use.
func levelFilter() *logutils.LevelFilter {
	return &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERR"},
		MinLevel: "INFO",
		Writer:   ioutil.Discard,
	}
}
