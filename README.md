# Caddy Meter Module

A Caddy module for defining and recording custom Prometheus metrics (counters and histograms) directly from your Caddy configuration.

## Features
- Define custom Prometheus counters and histograms in the Caddy global options.
- Record metrics in HTTP handlers using the `meter` directive.
- Supports labels (with Caddy placeholders) and custom histogram buckets.

## Installation

Add this module to your Caddy build or plugin set. (Refer to Caddy's documentation for custom builds.)

## Configuration

### 1. Define Metrics (Global Options)

Add a `meter` block to your global options in the Caddyfile:

```caddyfile
{
  meter {
    counter my_requests {
      description "Number of requests"
      labels {
        method {http.request.method}
        path {http.request.uri}
      }
    }
    histogram response_time {
      description "Response time in seconds"
      labels {
        method {http.request.method}
      }
      buckets 0.1 0.3 1.2 5.0
    }
  }
}
```

#### Metric Types
- `counter <name>`: Defines a counter metric.
- `histogram <name>`: Defines a histogram metric.

#### Metric Options
- `description`: Human-readable description for the metric.
- `name`: (optional) The exported name for the metric in Prometheus. If omitted, the metric's Caddy name is used.
- `labels`: Block of key-value pairs for Prometheus labels. Values can use Caddy placeholders.
- `buckets`: (histogram only) List of bucket boundaries (space-separated floats).
- `after_response`: (optional, for counters) Record metric after response is sent.
- `value`: (optional) Custom value for the metric (can use placeholders).

### 2. Use Metrics in HTTP Handlers

To record a metric, use the `meter` directive in your site block or route:

```caddyfile
route {
  meter my_requests
  respond "Hello, world!"
}
```

- `meter <metric_name>`: Increments or observes the named metric.
- You can optionally use a matcher: `meter @matcher <metric_name>`

### 3. Prometheus Integration

Metrics are registered with Prometheus and can be scraped from the Caddy Prometheus endpoint (if enabled).

### 4. Custom Placeholders

When using the `meter` directive, the following placeholders are available for use in labels and values:

- `{meter.response.status}`: The HTTP response status code (e.g., 200, 404).
- `{meter.response.size}`: The size of the response in bytes.
- `{meter.response.duration}`: The duration of the response in seconds (as a float).

These placeholders are especially useful for histograms and for labeling metrics with response details.

## Example

```caddyfile
{
  meter {
    counter api_hits {
      description "API hits"
      labels {
        endpoint {http.request.uri}
      }
    }
    histogram api_latency {
      description "API latency"
      labels {
        endpoint {http.request.uri}
      }
      buckets 0.05 0.1 0.5 1 2 5
    }
  }
}

:8080 {
  route {
    meter api_hits
    meter api_latency
    respond "API response"
  }
}
```

## Development

- Requires Go 1.18+ and Caddy v2.10.0+
- Main dependencies: [caddyserver/caddy](https://github.com/caddyserver/caddy), [prometheus/client_golang](https://github.com/prometheus/client_golang)

## License

MIT 