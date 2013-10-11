package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"
)

const eventScript = `#!/bin/sh
RESULT_FILE="%s"
echo $SERF_EVENT $SERF_USER_EVENT "$@" >>${RESULT_FILE}
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

func TestScriptEventHandler(t *testing.T) {
	script, results := testEventScript(t)

	h := &ScriptEventHandler{
		Scripts: []EventScript{
			{
				Event:  "*",
				Script: script,
			},
		},
	}

	event := serf.MemberEvent{
		Type: serf.EventMemberJoin,
		Members: []serf.Member{
			{
				Name: "foo",
				Addr: net.ParseIP("1.2.3.4"),
				Role: "bar",
			},
		},
	}

	discardLogger := log.New(os.Stderr, "", log.LstdFlags)
	err := h.HandleEvent(discardLogger, event)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	result, err := ioutil.ReadFile(results)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	expected := "member-join\nfoo\t1.2.3.4\tbar\n"
	if string(result) != expected {
		t.Fatalf("bad: %#v. Expected: %#v", string(result), expected)
	}
}
