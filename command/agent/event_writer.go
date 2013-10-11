package agent

// EventWriter is an io.Writer that writes all of its output to
// the event log of an agent.
type EventWriter struct {
	Agent *Agent
}

func (w *EventWriter) Write(p []byte) (n int, err error) {
	n = len(p)

	// Strip off newlines at the end if there are any since we store
	// individual log lines in the agent.
	if p[n-1] == '\n' {
		p = p[:n-1]
	}

	w.Agent.storeLog(string(p))
	return
}
