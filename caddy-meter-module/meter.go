package meter

import (
	"strconv"
	"sync"

	"errors"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(MeterModule{})
	httpcaddyfile.RegisterGlobalOption("meter", parseMeter)
}

type Label struct {
	Name    string
	Matcher string // optional
	Value   string
}

type MeterModule struct {
	Metrics map[string]*Metric `json:"metrics,omitempty"`
	mu      sync.RWMutex
}

// CaddyModule returns the Caddy module information.
func (MeterModule) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "meter",
		New: func() caddy.Module { return new(MeterModule) },
	}
}

// Start implements caddy.App.
func (m *MeterModule) Start() error {
	caddy.Log().Info("Starting meter module")
	return nil
}

// Stop implements caddy.App.
func (m *MeterModule) Stop() error {
	caddy.Log().Info("Stopping meter module")
	// Deregister metrics here if needed
	return nil
}

// Provision sets up m.
func (m *MeterModule) Provision(ctx caddy.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.Metrics) == 0 {
		return nil
	}

	for _, m := range m.Metrics {
		labelNames := make([]string, 0, len(m.Labels))
		for _, l := range m.Labels {
			labelNames = append(labelNames, l.Name)
		}

		registry := ctx.GetMetricsRegistry()

		switch m.Type {
		case MetricTypeCounter:
			m.requestCounter = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: m.ExportName,
					Help: m.Description,
				},
				labelNames,
			)
			err := registry.Register(m.requestCounter)
			if err != nil {
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					m.requestCounter = are.ExistingCollector.(*prometheus.CounterVec)
				} else {
					return err
				}
			}
		case "histogram":
			m.responseTime = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    m.ExportName,
					Help:    m.Description,
					Buckets: m.Buckets,
				},
				labelNames,
			)
			err := registry.Register(m.responseTime)
			if err != nil {
				if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
					m.responseTime = are.ExistingCollector.(*prometheus.HistogramVec)
				} else {
					return err
				}
			}
		default:
			return errors.New("unknown metric type: " + string(m.Type))
		}
	}

	return nil
}

func parseMeter(d *caddyfile.Dispenser, existingVal any) (any, error) {
	if m, ok := existingVal.(*MeterModule); ok {
		err := m.UnmarshalCaddyfile(d)
		return m, err
	}
	var m = new(MeterModule)
	err := m.UnmarshalCaddyfile(d)
	return httpcaddyfile.App{
		Name:  "meter",
		Value: caddyconfig.JSON(m, nil),
	}, err
}

// UnmarshalCaddyfile sets up the handler from Caddyfile tokens. Syntax:
//
//	meter {
//	  counter <metric_name> {
//	    description <description>
//	    labels {
//		  <label_name> <label_value>
//	    }
//	  }
//	  histogram <metric_name> {
//	    description <description>
//	    labels {
//		  <label_name> <label_value>
//	    }
//	    buckets <bucket_list>
//	  }
//	}
func (mm *MeterModule) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	mm.Metrics = make(map[string]*Metric) // reset
	if !d.Next() {
		caddy.Log().Error("expected 'meter' block")
		return d.Err("expected 'meter' block")
	}
	if !d.NextBlock(0) {
		caddy.Log().Error("expected block for 'meter'")
		return d.Err("expected block for 'meter'")
	}
	for {
		if d.Val() == "" {
			break
		}
		if d.Val() == "}" {
			break
		}
		metricType := MetricType(d.Val())
		switch metricType {
		case MetricTypeCounter, MetricTypeHistogram:
			m := &Metric{Type: metricType}

			if metricType == MetricTypeHistogram {
				// histogram metrics are always after response
				m.AfterResponse = true
			}

			// Parse optional matcher
			// Parse metric name
			if !d.NextArg() {
				caddy.Log().Error("expected metric name")
				return d.Err("expected metric name")
			}
			m.Name = d.Val()
			if m.Name == "" {
				caddy.Log().Error("metric name is required")
				return d.Err("metric name is required")
			}
			// Parse metric block
			if !d.NextBlock(1) {
				caddy.Log().Error("expected block for %s %s", zap.String("metricType", string(metricType)), zap.String("metricName", m.Name))
				return d.Errf("expected block for %s %s", metricType, m.Name)
			}
			for {
				if d.Val() == "" {
					break
				}
				switch d.Val() {
				case "description":
					if !d.NextArg() {
						return d.Err("description requires a value")
					}
					m.Description = d.Val()
				case "name":
					if !d.NextArg() {
						return d.Err("name requires a value")
					}
					m.ExportName = d.Val()
				case "after_response":
					m.AfterResponse = true
				case "value":
					if !d.NextArg() {
						return d.Err("value requires a value")
					}
					m.Value = d.Val()
				case "labels":
					labels, err := parseLabelsBlock(d)
					if err != nil {
						return err
					}
					m.Labels = labels
				case "buckets":
					if metricType != "histogram" {
						return d.Err("buckets only valid for histogram")
					}
					if !d.NextArg() {
						return d.Err("buckets requires a list of values")
					}
					var buckets []float64
					for {
						val := d.Val()
						if val == "" {
							break
						}
						f, err := parseFloat(val)
						if err != nil {
							return d.Errf("invalid bucket value: %s", val)
						}
						buckets = append(buckets, f)
						if !d.NextArg() {
							break
						}
					}
					m.Buckets = buckets
				default:
					caddy.Log().Error("unexpected block", zap.String("block", d.Val()))
					return d.Errf("unexpected block: %s", d.Val())
				}
				if !d.NextBlock(1) {
					break
				}
			}
			if _, ok := mm.Metrics[m.Name]; ok {
				return d.Errf("metric %s already defined", m.Name)
			}
			if m.ExportName == "" {
				m.ExportName = m.Name
			}
			mm.Metrics[m.Name] = m
		default:
			caddy.Log().Error("unexpected directive in meter", zap.String("directive", d.Val()))
			return d.Errf("unexpected directive in meter: %s", d.Val())
		}
		if !d.Next() {
			break
		}
	}
	return nil
}

func parseLabelsBlock(d *caddyfile.Dispenser) ([]Label, error) {
	var labels []Label
	for d.NextBlock(2) {
		if d.Val() == "}" {
			break
		}
		label := Label{}
		label.Name = d.Val()
		if !d.NextArg() {
			return nil, d.Err("label requires a value")
		}
		label.Value = d.Val()
		labels = append(labels, label)
	}
	return labels, nil
}

func parseFloat(s string) (float64, error) {
	// Simple wrapper for parsing float64
	return strconv.ParseFloat(s, 64)
}

// Interface guards
var (
	_ caddy.App             = (*MeterModule)(nil)
	_ caddy.Provisioner     = (*MeterModule)(nil)
	_ caddyfile.Unmarshaler = (*MeterModule)(nil)
)
