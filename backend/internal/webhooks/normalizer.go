package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// AlertNormalizer converts raw webhook payloads into a NormalizedAlert
type AlertNormalizer interface {
	Normalize(raw json.RawMessage) (*models.NormalizedAlert, error)
	Source() string
}

// GetNormalizer returns the appropriate normalizer for the given source
func GetNormalizer(source string) (AlertNormalizer, error) {
	switch source {
	case "datadog":
		return &DatadogNormalizer{}, nil
	case "prometheus_alertmanager":
		return &PrometheusNormalizer{}, nil
	case "grafana":
		return &GrafanaNormalizer{}, nil
	case "otel":
		return &OTelNormalizer{}, nil
	default:
		return nil, fmt.Errorf("unsupported source: %s", source)
	}
}

// --- Datadog ---
type datadogPayload struct {
	AlertID    string `json:"alert_id"`
	AlertTitle string `json:"alert_title"`
	AlertType  string `json:"alert_type"`
	Hostname   string `json:"hostname"`
	Priority   string `json:"priority"`
	Tags       string `json:"tags"`
}

type DatadogNormalizer struct{}

func (d *DatadogNormalizer) Source() string { return "datadog" }

func (d *DatadogNormalizer) Normalize(raw json.RawMessage) (*models.NormalizedAlert, error) {
	var p datadogPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal datadog payload: %w", err)
	}

	tags := make(map[string]string)
	service := ""
	for _, tag := range strings.Split(p.Tags, ",") {
		parts := strings.SplitN(strings.TrimSpace(tag), ":", 2)
		if len(parts) == 2 {
			tags[parts[0]] = parts[1]
			if parts[0] == "service" {
				service = parts[1]
			}
		}
	}

	severity := mapDatadogSeverity(p.Priority)

	return &models.NormalizedAlert{
		Title:       p.AlertTitle,
		Description: fmt.Sprintf("Datadog alert %s on %s", p.AlertID, p.Hostname),
		Severity:    severity,
		Source:      "datadog",
		Service:     service,
		Tags:        tags,
		StartedAt:   time.Now(),
		Labels:      map[string]string{"hostname": p.Hostname, "alert_type": p.AlertType},
	}, nil
}

func mapDatadogSeverity(priority string) string {
	switch strings.ToLower(priority) {
	case "critical", "p1":
		return "critical"
	case "high", "p2":
		return "high"
	case "medium", "p3", "normal":
		return "medium"
	default:
		return "low"
	}
}

// --- Prometheus AlertManager ---
type prometheusPayload struct {
	Alerts []prometheusAlert `json:"alerts"`
}

type prometheusAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
}

type PrometheusNormalizer struct{}

func (p *PrometheusNormalizer) Source() string { return "prometheus_alertmanager" }

func (p *PrometheusNormalizer) Normalize(raw json.RawMessage) (*models.NormalizedAlert, error) {
	var payload prometheusPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal prometheus payload: %w", err)
	}

	if len(payload.Alerts) == 0 {
		return nil, fmt.Errorf("no alerts in prometheus payload")
	}

	alert := payload.Alerts[0]
	severity := mapPrometheusSeverity(alert.Labels["severity"])

	return &models.NormalizedAlert{
		Title:       alert.Labels["alertname"],
		Description: alert.Annotations["summary"],
		Severity:    severity,
		Source:      "prometheus_alertmanager",
		Service:     alert.Labels["service"],
		Tags:        alert.Labels,
		StartedAt:   alert.StartsAt,
		Labels:      alert.Labels,
	}, nil
}

func mapPrometheusSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "critical"
	case "high", "warning":
		return "high"
	case "medium", "info":
		return "medium"
	default:
		return "low"
	}
}

// --- Grafana ---
type grafanaPayload struct {
	Title       string                   `json:"title"`
	State       string                   `json:"state"`
	Message     string                   `json:"message"`
	EvalMatches []map[string]interface{} `json:"evalMatches"`
	Tags        map[string]string        `json:"tags"`
}

type GrafanaNormalizer struct{}

func (g *GrafanaNormalizer) Source() string { return "grafana" }

func (g *GrafanaNormalizer) Normalize(raw json.RawMessage) (*models.NormalizedAlert, error) {
	var p grafanaPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal grafana payload: %w", err)
	}

	severity := "medium"
	if s, ok := p.Tags["severity"]; ok {
		severity = mapGrafanaSeverity(s)
	}

	service := p.Tags["service"]

	return &models.NormalizedAlert{
		Title:       p.Title,
		Description: p.Message,
		Severity:    severity,
		Source:      "grafana",
		Service:     service,
		Tags:        p.Tags,
		StartedAt:   time.Now(),
		Labels:      p.Tags,
	}, nil
}

func mapGrafanaSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

// --- OTel ---
type otelPayload struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []otelAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				Name       string          `json:"name"`
				Status     otelStatus      `json:"status"`
				Attributes []otelAttribute `json:"attributes"`
				StartTime  string          `json:"startTimeUnixNano"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
	// Simplified: also accept flat format
	AlertName   string            `json:"alert_name"`
	Severity    string            `json:"severity"`
	ServiceName string            `json:"service_name"`
	Description string            `json:"description"`
	Attributes  map[string]string `json:"attributes"`
	StartTime   string            `json:"start_time"`
}

type otelAttribute struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type otelStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type OTelNormalizer struct{}

func (o *OTelNormalizer) Source() string { return "otel" }

func (o *OTelNormalizer) Normalize(raw json.RawMessage) (*models.NormalizedAlert, error) {
	var p otelPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("unmarshal otel payload: %w", err)
	}

	// Use flat format if provided
	if p.AlertName != "" {
		tags := p.Attributes
		if tags == nil {
			tags = make(map[string]string)
		}

		startedAt := time.Now()
		if p.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, p.StartTime); err == nil {
				startedAt = t
			}
		}

		return &models.NormalizedAlert{
			Title:       p.AlertName,
			Description: p.Description,
			Severity:    mapOTelSeverity(p.Severity),
			Source:      "otel",
			Service:     p.ServiceName,
			Tags:        tags,
			StartedAt:   startedAt,
			Labels:      tags,
		}, nil
	}

	// Try resourceSpans format
	if len(p.ResourceSpans) > 0 && len(p.ResourceSpans[0].ScopeSpans) > 0 && len(p.ResourceSpans[0].ScopeSpans[0].Spans) > 0 {
		span := p.ResourceSpans[0].ScopeSpans[0].Spans[0]
		return &models.NormalizedAlert{
			Title:       span.Name,
			Description: span.Status.Message,
			Severity:    mapOTelSeverity(span.Status.Code),
			Source:      "otel",
			Service:     "",
			Tags:        make(map[string]string),
			StartedAt:   time.Now(),
			Labels:      make(map[string]string),
		}, nil
	}

	return nil, fmt.Errorf("unable to parse OTel payload")
}

func mapOTelSeverity(code string) string {
	switch strings.ToLower(code) {
	case "critical", "fatal":
		return "critical"
	case "error", "high":
		return "high"
	case "warn", "warning", "medium":
		return "medium"
	default:
		return "low"
	}
}
