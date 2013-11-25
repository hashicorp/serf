package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"log"
)

// eventStream is used to stream events to a client over IPC
type eventStream struct {
	client  *IPCClient
	eventCh chan serf.Event
	filters []EventFilter
	logger  *log.Logger
	seq     int
}

func newEventStream(client *IPCClient, filters []EventFilter, seq int, logger *log.Logger) *eventStream {
	es := &eventStream{
		client:  client,
		eventCh: make(chan serf.Event, 512),
		filters: filters,
		logger:  logger,
		seq:     seq,
	}
	go es.stream()
	return es
}

func (es *eventStream) HandleEvent(e serf.Event) {
	// Check the event
	for _, f := range es.filters {
		if f.Invoke(e) {
			goto HANDLE
		}
	}
	return

	// Do a non-blocking send
HANDLE:
	select {
	case es.eventCh <- e:
	default:
		es.logger.Printf("[WARN] Dropping event to %v", es.client.conn)
	}
}

func (es *eventStream) Stop() {
	close(es.eventCh)
}

func (es *eventStream) stream() {
	var err error
	for event := range es.eventCh {
		switch e := event.(type) {
		case serf.MemberEvent:
			err = es.sendMemberEvent(e)
		case serf.UserEvent:
			err = es.sendUserEvent(e)
		default:
			err = fmt.Errorf("Unknown event type: %s", event.EventType().String())
		}
		if err != nil {
			es.logger.Printf("[ERR] Failed to stream event to %v: %v",
				es.client.conn, err)
			return
		}
	}
}

// sendMemberEvent is used to send a single member event
func (es *eventStream) sendMemberEvent(me serf.MemberEvent) error {
	members := make([]member, 0, len(me.Members))
	for _, m := range me.Members {
		sm := member{
			Name:        m.Name,
			Addr:        m.Addr,
			Port:        m.Port,
			Role:        m.Role,
			Status:      m.Status.String(),
			ProtocolMin: m.ProtocolMin,
			ProtocolMax: m.ProtocolMax,
			ProtocolCur: m.ProtocolCur,
			DelegateMin: m.DelegateMin,
			DelegateMax: m.DelegateMax,
			DelegateCur: m.DelegateCur,
		}
		members = append(members, sm)
	}

	rec := memberEventRecord{
		Seq:     es.seq,
		Event:   me.String(),
		Members: members,
	}
	return es.client.send(&rec)
}

// sendUserEvent is used to send a single user event
func (es *eventStream) sendUserEvent(ue serf.UserEvent) error {
	rec := userEventRecord{
		Seq:      es.seq,
		Event:    ue.EventType().String(),
		LTime:    ue.LTime,
		Name:     ue.Name,
		Payload:  ue.Payload,
		Coalesce: ue.Coalesce,
	}
	return es.client.send(&rec)
}
