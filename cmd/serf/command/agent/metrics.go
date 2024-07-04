package agent

import (
	"fmt"
	"slices"
	"sort"
	"sync/atomic"

	"github.com/hashicorp/serf/serf"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	EXPORTER_ROLE = "metrics_exporter"
	ROLE_TAG      = "role"
)

type MetricsEventHandler struct {
	agent                   *Agent
	serfEventReceived       *prometheus.CounterVec
	serfNodeStatus          *prometheus.GaugeVec
	serfMetricsExporterRole prometheus.Gauge
	// metricExporter is true when this node is the elected exporter.
	// Written by the event loop goroutine, read by the Prometheus scrape goroutine.
	metricExporter atomic.Bool
}

func NewMetricsEventHandler(a *Agent) *MetricsEventHandler {
	handler := &MetricsEventHandler{
		agent: a,
		serfEventReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "serf_event_received",
			Help: "Number of serf events received, by event type",
		}, []string{"event_type"}),
		serfNodeStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "serf_node_status",
			Help: fmt.Sprintf("Status of each serf cluster member as seen by the elected exporter: %d=alive, %d=leaving, %d=left, %d=failed",
				serf.StatusAlive, serf.StatusLeaving, serf.StatusLeft, serf.StatusFailed),
		}, []string{"node"}),
		serfMetricsExporterRole: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "serf_metrics_exporter",
			Help: "1 if this node is the elected metrics exporter for the cluster, 0 otherwise",
		}),
	}

	prometheus.MustRegister(handler)
	return handler
}

// HandleEvent implements EventHandler. Election is re-evaluated only on
// membership events. User events and queries do not change cluster membership
// and should not trigger tag gossip.
func (m *MetricsEventHandler) HandleEvent(e serf.Event) {
	m.serfEventReceived.WithLabelValues(e.EventType().String()).Inc()

	switch e.EventType() {
	case serf.EventMemberJoin,
		serf.EventMemberLeave,
		serf.EventMemberFailed,
		serf.EventMemberUpdate,
		serf.EventMemberReap:
		m.evaluateExporterRole()
	}
}

// evaluateExporterRole checks whether this node should be the metrics exporter
// and updates the serf role tag only when the role actually changes, to avoid
// unnecessary gossip churn.
//
// A node that sees fewer than 2 alive peers is considered potentially isolated
// (split-brain) and must not claim to be the authoritative cluster exporter.
func (m *MetricsEventHandler) evaluateExporterRole() {
	exporters := m.exporters()
	aliveNodes := m.aliveNodes()
	localName := m.agent.serf.LocalMember().Name
	isExporter := slices.Contains(exporters, localName)

	// Step down if isolated: seeing only yourself means you may be partitioned
	// from the rest of the cluster and cannot speak for it.
	if len(aliveNodes) < 2 {
		if isExporter {
			m.setExporterRole(false)
		}
		return
	}

	switch {
	case len(exporters) == 0:
		// No exporter in the cluster: elect the alphabetically first alive node.
		m.electMetricsExporter(aliveNodes)

	case len(exporters) > 1 && isExporter:
		// Multiple exporters (transient during gossip convergence):
		// all but the highest-priority one step down.
		if !m.hasPriority(exporters) {
			m.setExporterRole(false)
		}
	}
}

func (m *MetricsEventHandler) electMetricsExporter(aliveNodes []string) {
	sort.Strings(aliveNodes)
	if aliveNodes[0] == m.agent.serf.LocalMember().Name {
		m.setExporterRole(true)
	}
}

// setExporterRole updates the serf role tag and the local flag only when the
// role actually changes, to avoid gossip churn on every membership event.
func (m *MetricsEventHandler) setExporterRole(exporter bool) {
	if exporter {
		if m.metricExporter.Load() {
			return // already exporter, no change
		}
		m.agent.serf.SetTags(map[string]string{ROLE_TAG: EXPORTER_ROLE})
		m.metricExporter.Store(true)
	} else {
		if !m.metricExporter.Load() {
			return // already not exporter, no change
		}
		// Copy the tags map before mutating to avoid modifying serf's internal state.
		tags := make(map[string]string)
		for k, v := range m.agent.serf.LocalMember().Tags {
			tags[k] = v
		}
		delete(tags, ROLE_TAG)
		m.agent.serf.SetTags(tags)
		m.metricExporter.Store(false)
	}
}

func (m *MetricsEventHandler) aliveNodes() []string {
	var aliveNodes []string
	for _, member := range m.agent.serf.Members() {
		if member.Status == serf.StatusAlive {
			aliveNodes = append(aliveNodes, member.Name)
		}
	}
	return aliveNodes
}

func (m *MetricsEventHandler) exporters() []string {
	var exporters []string
	for _, member := range m.agent.serf.Members() {
		if role, ok := member.Tags[ROLE_TAG]; ok && role == EXPORTER_ROLE && member.Status == serf.StatusAlive {
			exporters = append(exporters, member.Name)
		}
	}
	return exporters
}

func (m *MetricsEventHandler) hasPriority(exporters []string) bool {
	sort.Strings(exporters)
	return exporters[0] == m.agent.serf.LocalMember().Name
}

func (m *MetricsEventHandler) Collect(ch chan<- prometheus.Metric) {
	if m.metricExporter.Load() {
		m.serfMetricsExporterRole.Set(1)
		// Reset before re-populating so nodes that left the cluster don't
		// leave behind stale time series.
		m.serfNodeStatus.Reset()
		for _, member := range m.agent.serf.Members() {
			if member.Status == serf.StatusNone {
				continue // transient/unknown omit rather than emit a misleading value
			}
			m.serfNodeStatus.WithLabelValues(member.Name).Set(float64(member.Status))
		}
		m.serfNodeStatus.Collect(ch)
	} else {
		m.serfMetricsExporterRole.Set(0)
	}

	m.serfEventReceived.Collect(ch)
	m.serfMetricsExporterRole.Collect(ch)
}

func (m *MetricsEventHandler) Describe(ch chan<- *prometheus.Desc) {
	if m.metricExporter.Load() {
		m.serfNodeStatus.Describe(ch)
	}
	m.serfEventReceived.Describe(ch)
	m.serfMetricsExporterRole.Describe(ch)
}
