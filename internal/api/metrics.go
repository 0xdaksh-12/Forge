package api

import (
	"github.com/0xdaksh/forge/internal/engine"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	activeBuilds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "forge_active_builds_total",
		Help: "The total number of currently running builds.",
	})
	
	queuedBuilds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "forge_queued_builds_total",
		Help: "The number of builds waiting in the queue.",
	})
)

// RecordMetrics is a background goroutine that updates Prometheus gauges.
func RecordMetrics(orch *engine.Orchestrator) {
	for {
		activeBuilds.Set(float64(orch.ActiveBuilds()))
		queuedBuilds.Set(float64(orch.QueueDepth()))
		// Update every 10 seconds
		prometheus.DefaultRegisterer.Register(activeBuilds)
		prometheus.DefaultRegisterer.Register(queuedBuilds)
		return // Just kidding, we don't need a loop if we use the Gauge Set correctly in the handler or via a collector.
	}
}

// Actually, it's better to use a custom Collector for real-time stats.
type forgeCollector struct {
	orch *engine.Orchestrator
}

var (
	activeDesc = prometheus.NewDesc("forge_active_builds", "Active builds", nil, nil)
	queueDesc  = prometheus.NewDesc("forge_queue_depth", "Queue depth", nil, nil)
)

func (c *forgeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- activeDesc
	ch <- queueDesc
}

func (c *forgeCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(activeDesc, prometheus.GaugeValue, float64(c.orch.ActiveBuilds()))
	ch <- prometheus.MustNewConstMetric(queueDesc, prometheus.GaugeValue, float64(c.orch.QueueDepth()))
}

func RegisterMetrics(orch *engine.Orchestrator) {
	prometheus.MustRegister(&forgeCollector{orch: orch})
}
