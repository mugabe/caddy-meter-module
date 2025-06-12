package meter

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(UseMeter{})

	httpcaddyfile.RegisterHandlerDirective("meter", parseUseMeter)
	httpcaddyfile.RegisterDirectiveOrder("meter", httpcaddyfile.After, "templates")
}

// UseMeter is a handler that records a specified metric by name.
type UseMeter struct {
	Matcher    string // optional matcher (e.g., @foo)
	MetricName string // metric name to use

	useMetric *Metric
}

func (UseMeter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.meter",
		New: func() caddy.Module { return new(UseMeter) },
	}
}

func (h *UseMeter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// Syntax: meter [@matcher] <metric_name>
	if !d.Next() {
		return d.Err("expected directive arguments")
	}
	if d.NextArg() && len(d.Val()) > 0 && d.Val()[0] == '@' {
		h.Matcher = d.Val()
		if !d.NextArg() {
			return d.Err("expected metric name after matcher")
		}
	}
	h.MetricName = d.Val()
	if h.MetricName == "" {
		return d.Err("metric name is required")
	}
	return nil
}

func (h *UseMeter) Provision(ctx caddy.Context) error {
	metricsApp, err := ctx.AppIfConfigured("meter")
	if err != nil {
		return err
	}

	// The global options are a slice of *Metrics
	metricsList, ok := metricsApp.(*MeterModule)
	if !ok {
		caddy.Log().Error("metrics global option has unexpected type")
		return err
	}

	useMetric, ok := metricsList.Metrics[h.MetricName]
	if !ok {
		caddy.Log().Error("metric not found", zap.String("name", h.MetricName))
		return errors.New("metric " + h.MetricName + " not found")
	}

	h.useMetric = useMetric

	return nil
}

func (h *UseMeter) report(replacer *caddy.Replacer, value float64) {
	labelValues := make([]string, len(h.useMetric.Labels))
	for i, label := range h.useMetric.Labels {
		labelValues[i] = replacer.ReplaceKnown(label.Value, "<empty>")
	}
	switch h.useMetric.Type {
	case MetricTypeCounter:
		if value == 0 {
			value = 1
		}
		h.useMetric.requestCounter.WithLabelValues(labelValues...).Add(value)
	case MetricTypeHistogram:
		h.useMetric.responseTime.WithLabelValues(labelValues...).Observe(value)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode     int
	responseLength int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Status() int {
	return w.statusCode
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.responseLength += n
	return n, err
}

func (h *UseMeter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	replacer := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	if !h.useMetric.AfterResponse {
		fValue := 1.0
		if h.useMetric.Value != "" {
			sValue := replacer.ReplaceAll(h.useMetric.Value, "1")
			var err error
			fValue, err = strconv.ParseFloat(sValue, 64)
			if err != nil {
				fValue = 0.0
			}
		} else if h.useMetric.Type == MetricTypeHistogram {
			fValue = 0.0
		}

		h.report(replacer, fValue)
		return next.ServeHTTP(w, r)
	}

	wrap := &responseWriter{ResponseWriter: w}
	start := time.Now()
	res := next.ServeHTTP(wrap, r)
	duration := time.Since(start)

	replacer.Set("meter.response.status", strconv.Itoa(wrap.Status()))
	replacer.Set("meter.response.size", strconv.Itoa(wrap.responseLength))
	replacer.Set("meter.response.duration", strconv.FormatFloat(duration.Seconds(), 'f', -1, 64))

	fValue := 0.0
	if h.useMetric.Value != "" {
		sValue := replacer.ReplaceAll(h.useMetric.Value, "1")
		var err error
		fValue, err = strconv.ParseFloat(sValue, 64)
		if err != nil {
			fValue = 0.0
		}
	} else if h.useMetric.Type == MetricTypeHistogram {
		fValue = duration.Seconds()
	}

	h.report(replacer, fValue)
	return res
}

func parseUseMeter(helper httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var h UseMeter
	if err := h.UnmarshalCaddyfile(helper.Dispenser); err != nil {
		return nil, err
	}
	return &h, nil
}

// Interface guards
var (
	_ caddyhttp.MiddlewareHandler = (*UseMeter)(nil)
	_ caddyfile.Unmarshaler       = (*UseMeter)(nil)
)
