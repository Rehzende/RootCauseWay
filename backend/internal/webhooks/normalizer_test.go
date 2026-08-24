package webhooks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatadogNormalizer(t *testing.T) {
	raw := json.RawMessage(`{"alert_id": "123", "alert_title": "High CPU", "alert_type": "error", "hostname": "web-1", "priority": "critical", "tags": "service:api,env:prod"}`)

	n := &DatadogNormalizer{}
	result, err := n.Normalize(raw)

	require.NoError(t, err)
	assert.Equal(t, "High CPU", result.Title)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, "datadog", result.Source)
	assert.Equal(t, "api", result.Service)
	assert.Equal(t, "api", result.Tags["service"])
	assert.Equal(t, "prod", result.Tags["env"])
	assert.Equal(t, "web-1", result.Labels["hostname"])
}

func TestPrometheusNormalizer(t *testing.T) {
	raw := json.RawMessage(`{"alerts": [{"status": "firing", "labels": {"alertname": "HighCPU", "severity": "critical", "service": "api"}, "annotations": {"summary": "CPU > 90%"}, "startsAt": "2026-01-01T00:00:00Z"}]}`)

	n := &PrometheusNormalizer{}
	result, err := n.Normalize(raw)

	require.NoError(t, err)
	assert.Equal(t, "HighCPU", result.Title)
	assert.Equal(t, "CPU > 90%", result.Description)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, "prometheus_alertmanager", result.Source)
	assert.Equal(t, "api", result.Service)
}

func TestPrometheusNormalizer_EmptyAlerts(t *testing.T) {
	raw := json.RawMessage(`{"alerts": []}`)

	n := &PrometheusNormalizer{}
	_, err := n.Normalize(raw)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no alerts")
}

func TestGrafanaNormalizer(t *testing.T) {
	raw := json.RawMessage(`{"title": "High CPU", "state": "alerting", "message": "CPU usage above threshold", "evalMatches": [{"metric": "cpu", "value": 95}], "tags": {"service": "api", "severity": "critical"}}`)

	n := &GrafanaNormalizer{}
	result, err := n.Normalize(raw)

	require.NoError(t, err)
	assert.Equal(t, "High CPU", result.Title)
	assert.Equal(t, "CPU usage above threshold", result.Description)
	assert.Equal(t, "critical", result.Severity)
	assert.Equal(t, "grafana", result.Source)
	assert.Equal(t, "api", result.Service)
}

func TestOTelNormalizer(t *testing.T) {
	raw := json.RawMessage(`{"alert_name": "HighLatency", "severity": "high", "service_name": "api-gateway", "description": "P99 latency > 500ms", "attributes": {"env": "production"}, "start_time": "2026-01-01T00:00:00Z"}`)

	n := &OTelNormalizer{}
	result, err := n.Normalize(raw)

	require.NoError(t, err)
	assert.Equal(t, "HighLatency", result.Title)
	assert.Equal(t, "P99 latency > 500ms", result.Description)
	assert.Equal(t, "high", result.Severity)
	assert.Equal(t, "otel", result.Source)
	assert.Equal(t, "api-gateway", result.Service)
	assert.Equal(t, "production", result.Tags["env"])
}

func TestGetNormalizer(t *testing.T) {
	tests := []struct {
		source string
		ok     bool
	}{
		{"datadog", true},
		{"prometheus_alertmanager", true},
		{"grafana", true},
		{"otel", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			n, err := GetNormalizer(tt.source)
			if tt.ok {
				assert.NoError(t, err)
				assert.Equal(t, tt.source, n.Source())
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestDatadogNormalizer_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	n := &DatadogNormalizer{}
	_, err := n.Normalize(raw)
	assert.Error(t, err)
}
