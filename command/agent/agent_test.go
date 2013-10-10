package agent

import (
	"fmt"
	"github.com/hashicorp/serf/testutil"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

const eventScript = `#!/bin/sh
RESULT_FILE="%s"
echo $SERF_EVENT "$@" >>${RESULT_FILE}
while read line; do
	printf "${line}\n" >>${RESULT_FILE}
done
`

// testEventScript creates an event script that can be used with the
// agent. It returns the path to the event script itself and a path to
// the file that will contain the events that that script receives.
func testEventScript(t *testing.T) (string, string) {
	scriptFile, err := ioutil.TempFile("", "serf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer scriptFile.Close()

	if err := scriptFile.Chmod(0755); err != nil {
		t.Fatalf("err: %s", err)
	}

	resultFile, err := ioutil.TempFile("", "serf-result")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer resultFile.Close()

	_, err = scriptFile.Write([]byte(
		fmt.Sprintf(eventScript, resultFile.Name())))
	if err != nil {
		t.Fatalf("err: %s")
	}

	return scriptFile.Name(), resultFile.Name()
}

func TestAgent_events(t *testing.T) {
	a1 := testAgent()
	defer a1.Shutdown()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	eventsCh := make(chan string, 64)
	prev := a1.NotifyEvents(eventsCh)
	defer a1.StopEvents(eventsCh)

	if len(prev) != 1 {
		t.Fatalf("bad: %d", len(prev))
	}

	a1.Join(nil)

	select {
	case e := <-eventsCh:
		if !strings.Contains(e, "join") {
			t.Fatalf("bad: %s", e)
		}
	case <-time.After(5 * time.Millisecond):
		t.Fatal("timeout")
	}
}

func TestAgent_eventScript(t *testing.T) {
	a1 := testAgent()
	a2 := testAgent()
	defer a1.Shutdown()
	defer a2.Shutdown()

	script, results := testEventScript(t)
	a1.EventScript = script

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := a2.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err := a1.Serf().Join([]string{a2.SerfConfig.MemberlistConfig.BindAddr})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Double yield here on purpose just ot be sure
	testutil.Yield()
	testutil.Yield()

	result, err := ioutil.ReadFile(results)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := fmt.Sprintf(
		"member-join\n%s\t%s\n"+
			"member-join\n%s\t%s\n",
		a1.SerfConfig.NodeName,
		a1.SerfConfig.MemberlistConfig.BindAddr,
		a2.SerfConfig.NodeName,
		a2.SerfConfig.MemberlistConfig.BindAddr)

	if string(result) != expected {
		t.Fatalf("bad: %#v. Expected: %#v", string(result), expected)
	}
}

func TestAgentShutdown_multiple(t *testing.T) {
	a := testAgent()
	if err := a.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	for i := 0; i < 5; i++ {
		if err := a.Shutdown(); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
}

func TestAgentShutdown_noStart(t *testing.T) {
	a := testAgent()
	if err := a.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}
}
