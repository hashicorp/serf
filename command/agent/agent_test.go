package agent

import (
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"path/filepath"
	"io/ioutil"
	"reflect"
	"os"
	"strings"
	"testing"
)

func TestAgent_eventHandler(t *testing.T) {
	a1 := testAgent(nil)
	defer a1.Shutdown()
	defer a1.Leave()

	handler := new(MockEventHandler)
	a1.RegisterEventHandler(handler)

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if len(handler.Events) != 1 {
		t.Fatalf("bad: %#v", handler.Events)
	}

	if handler.Events[0].EventType() != serf.EventMemberJoin {
		t.Fatalf("bad: %#v", handler.Events[0])
	}
}

func TestAgentShutdown_multiple(t *testing.T) {
	a := testAgent(nil)
	if err := a.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	for i := 0; i < 5; i++ {
		if err := a.Shutdown(); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
}

func TestAgentUserEvent(t *testing.T) {
	a1 := testAgent(nil)
	defer a1.Shutdown()
	defer a1.Leave()

	handler := new(MockEventHandler)
	a1.RegisterEventHandler(handler)

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := a1.UserEvent("deploy", []byte("foo"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	handler.Lock()
	defer handler.Unlock()

	if len(handler.Events) == 0 {
		t.Fatal("no events")
	}

	e, ok := handler.Events[len(handler.Events)-1].(serf.UserEvent)
	if !ok {
		t.Fatalf("bad: %#v", e)
	}

	if e.Name != "deploy" {
		t.Fatalf("bad: %#v", e)
	}

	if string(e.Payload) != "foo" {
		t.Fatalf("bad: %#v", e)
	}
}

func TestAgentQuery_BadPrefix(t *testing.T) {
	a1 := testAgent(nil)
	defer a1.Shutdown()
	defer a1.Leave()

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	_, err := a1.Query("_serf_test", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot contain") {
		t.Fatalf("err: %s", err)
	}
}

func TestAgentTagsFile(t *testing.T) {
	tags := map[string]string{
		"role": "webserver",
		"datacenter": "us-east",
	}

	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	config := serf.DefaultConfig()
	config.TagsFile = filepath.Join(td, "tags.json")

	a1 := testAgentWithConfig(config, nil)

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}
	defer a1.Shutdown()
	defer a1.Leave()

	testutil.Yield()

	err = a1.Serf().SetTags(tags)

	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	a2 := testAgentWithConfig(config, nil)

	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}
	defer a2.Shutdown()
	defer a2.Leave()

	testutil.Yield()

	m := a2.Serf().LocalMember()

	if !reflect.DeepEqual(m.Tags, tags) {
		t.Fatalf("tags not restored: %v", m.Tags)
	}
}
