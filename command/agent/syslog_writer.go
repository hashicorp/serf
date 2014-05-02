package agent

import (
	"bytes"
	"log/syslog"
	"sync"
)

type SyslogWriter struct {
	syslog *syslog.Writer
	once    sync.Once
}

func (s *SyslogWriter) Write(p []byte) (n int, err error) {
	s.once.Do(s.init)

	// Extract log level
	var level string
	x := bytes.IndexByte(p, '[')
	if x >= 0 {
		y := bytes.IndexByte(p[x:], ']')
		if y >= 0 {
			level = string(p[x+1 : x+y])
		}
	}

	// Each log level will be handled by a specific syslog function
	level2fn := map[string]func(string) error {
		"TRACE": s.syslog.Debug,
		"DEBUG": s.syslog.Info,
		"INFO": s.syslog.Notice,
		"WARN": s.syslog.Warning,
		"ERR": s.syslog.Err,
	}
	fn, ok := level2fn[level]
	if !ok {
		fn = s.syslog.Info
	}

	// Write
	return len(string(p)), fn(string(p))
}

func (s *SyslogWriter) init() {
	s.syslog, _ = syslog.New(syslog.LOG_INFO | syslog.LOG_DAEMON, "serf")
}
