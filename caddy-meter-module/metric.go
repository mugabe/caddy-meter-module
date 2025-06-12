package meter

import (
	"github.com/prometheus/client_golang/prometheus"
)

type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeHistogram MetricType = "histogram"
)

type Metric struct {
	Type          MetricType `json:"type"`                     // "counter" or "histogram"
	Name          string     `json:"name"`                     // metric name
	ExportName    string     `json:"export_name,omitempty"`    // export name
	Description   string     `json:"description,omitempty"`    // description string
	Labels        []Label    `json:"labels,omitempty"`         // label definitions
	Buckets       []float64  `json:"buckets,omitempty"`        // only for histograms
	AfterResponse bool       `json:"after_response,omitempty"` // wait for response
	Value         string     `json:"value,omitempty"`          // value to use for the metric

	requestCounter *prometheus.CounterVec   // for counters
	responseTime   *prometheus.HistogramVec // for histograms
}
