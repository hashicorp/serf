package agent

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/serf/serf"
	"io"
	"log"
	"os/exec"
	"strings"
)

// invokeEventScript will execute the given event script with the given
// event.
func (a *Agent) invokeEventScript(script string, event serf.Event) error {
	var output bytes.Buffer
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Args[0] = "serf-event"
	cmd.Env = append(cmd.Env, "SERF_EVENT="+event.EventType().String())
	cmd.Stderr = &output
	cmd.Stdout = &output

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	switch e := event.(type) {
	case serf.MemberEvent:
		go memberEventStdin(a.logger, stdin, &e)
	case serf.UserEvent:
		cmd.Env = append(cmd.Env, "SERF_USER_EVENT="+e.Name)
		go userEventStdin(a.logger, stdin, &e)
	default:
		return fmt.Errorf("Unknown event type: %s", event.EventType().String())
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	err = cmd.Wait()
	a.logger.Printf("[DEBUG] Event '%s' script output: %s",
		event.EventType().String(), output.String())

	if err != nil {
		return err
	}

	// TODO(mitchellh): log stdout/stderr
	return nil
}

// eventClean cleans a value to be a parameter in an event line.
func eventClean(v string) string {
	v = strings.Replace(v, "\t", "\\t", -1)
	v = strings.Replace(v, "\n", "\\n", -1)
	return v
}

func memberEventStdin(logger *log.Logger, stdin io.WriteCloser, e *serf.MemberEvent) {
	defer stdin.Close()
	for _, member := range e.Members {
		_, err := stdin.Write([]byte(fmt.Sprintf(
			"%s\t%s\t%s\n",
			eventClean(member.Name),
			member.Addr.String(),
			eventClean(member.Role))))
		if err != nil {
			return
		}
	}
}

func userEventStdin(logger *log.Logger, stdin io.WriteCloser, e *serf.UserEvent) {
	defer stdin.Close()
	if _, err := stdin.Write(e.Payload); err != nil {
		logger.Printf("[ERR] Error writing user event payload: %s", err)
		return
	}
}
