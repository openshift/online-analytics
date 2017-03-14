package useranalytics

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/openshift/origin/pkg/api/install"
)

type MetricsServer struct {
	Config       MetricsConfig
	WoopraClient *WoopraDestination
	Controller   *AnalyticsController
}

type MetricsConfig struct {
	BindAddr       string `json:"bindAddr" yaml:"bindAddr"`
	CollectRuntime bool   `json:"collectRuntime" yaml:"collectRuntime"`
	CollectWoopra  bool   `json:"collectWoopra" yaml:"collectWoopra"`
	CollectQueue   bool   `json:"collectQueue" yaml:"collectQueue"`
}

const MetricsEndpoint = "/metrics"

var WoopraLatencyMetric = prometheus.NewDesc(
	"analytics_woopra_latency_seconds",
	"Latency to the Woopra API endpoint in seconds, 0 representing no connection.",
	[]string{},
	prometheus.Labels{},
)

var QueueSizeMetric = prometheus.NewDesc(
	"analytics_queue_size_events",
	"Number of events pending in analytics controller queue.",
	[]string{},
	prometheus.Labels{},
)

var EventsHandledMetric = prometheus.NewDesc(
	"analytics_events_handled",
	"Number of events processed by analytics controller queue.",
	[]string{},
	prometheus.Labels{},
)

func (s *MetricsServer) Serve() error {
	registry := prometheus.NewRegistry()

	if s.Config.CollectRuntime {
		if err := registry.Register(prometheus.NewGoCollector()); err != nil {
			return err
		}
		if err := registry.Register(prometheus.NewProcessCollector(os.Getpid(), "")); err != nil {
			return err
		}
	}

	if s.Config.CollectWoopra {
		if err := registry.Register(NewWoopraCollector(s.WoopraClient)); err != nil {
			return err
		}
	}

	if s.Config.CollectQueue {
		if err := registry.Register(NewSizeCollector(s.Controller)); err != nil {
			return err
		}

		if err := registry.Register(NewEventsCollector(s.Controller)); err != nil {
			return err
		}
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog: &logWrapper{},
	})

	mux := http.NewServeMux()
	mux.Handle(MetricsEndpoint, handler)
	server := &http.Server{
		Addr:    s.Config.BindAddr,
		Handler: mux,
	}
	return server.ListenAndServe()
}

type logWrapper struct{}

func (l *logWrapper) Println(v ...interface{}) {
	glog.V(0).Info(v)
}

type WoopraCollector struct {
	client *WoopraDestination
}

type SizeCollector struct {
	controller *AnalyticsController
}

type EventsCollector struct {
	controller *AnalyticsController
}

func NewWoopraCollector(client *WoopraDestination) *WoopraCollector {
	return &WoopraCollector{
		client: client,
	}
}

func (c *WoopraCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- WoopraLatencyMetric
}

func (c *WoopraCollector) Collect(ch chan<- prometheus.Metric) {
	client := c.client
	pingUrl := fmt.Sprintf("http://www.woopra.com/track/ping?response=json&host=%s", client.Domain)
	start := time.Now()
	_, err := client.Client.Get(pingUrl)
	delta := float64(time.Now().Sub(start)) / float64(time.Second)
	if err != nil {
		delta = 0
	}

	ch <- prometheus.MustNewConstMetric(
		WoopraLatencyMetric,
		prometheus.GaugeValue,
		delta,
	)
}

func NewSizeCollector(controller *AnalyticsController) *SizeCollector {
	return &SizeCollector{
		controller: controller,
	}
}

func (c *SizeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- QueueSizeMetric
}

func (c *SizeCollector) Collect(ch chan<- prometheus.Metric) {
	size := float64(len(c.controller.queue.ListKeys()))
	ch <- prometheus.MustNewConstMetric(
		QueueSizeMetric,
		prometheus.GaugeValue,
		size,
	)
}

func NewEventsCollector(controller *AnalyticsController) *EventsCollector {
	return &EventsCollector{
		controller: controller,
	}
}

func (c *EventsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- EventsHandledMetric
}

func (c *EventsCollector) Collect(ch chan<- prometheus.Metric) {
	events := float64(c.controller.eventsHandled)
	ch <- prometheus.MustNewConstMetric(
		EventsHandledMetric,
		prometheus.GaugeValue,
		events,
	)
}
