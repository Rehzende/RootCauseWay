package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Observability Data Sources ---

type ObservabilitySource struct {
	ID                   uuid.UUID       `json:"id"`
	OrgID                uuid.UUID       `json:"org_id"`
	Name                 string          `json:"name"`
	SourceType           string          `json:"source_type"`
	BaseURL              string          `json:"base_url"`
	AuthType             string          `json:"auth_type"`
	AuthConfig           json.RawMessage `json:"auth_config"`
	Capabilities         json.RawMessage `json:"capabilities"`
	MonitoredSoftwareIDs json.RawMessage `json:"monitored_software_ids"`
	TimeoutSeconds       int             `json:"timeout_seconds"`
	VerifySSL            bool            `json:"verify_ssl"`
	CustomHeaders        json.RawMessage `json:"custom_headers"`
	Enabled              bool            `json:"enabled"`
	HealthStatus         string          `json:"health_status"`
	LastHealthCheck      *time.Time      `json:"last_health_check,omitempty"`
	Description          string          `json:"description"`
	Environment          string          `json:"environment"`
	Region               string          `json:"region"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type CreateObservabilitySourceRequest struct {
	Name                 string          `json:"name" binding:"required"`
	SourceType           string          `json:"source_type" binding:"required"`
	BaseURL              string          `json:"base_url" binding:"required"`
	AuthType             string          `json:"auth_type"`
	AuthConfig           json.RawMessage `json:"auth_config"`
	Capabilities         json.RawMessage `json:"capabilities"`
	MonitoredSoftwareIDs json.RawMessage `json:"monitored_software_ids"`
	TimeoutSeconds       int             `json:"timeout_seconds"`
	VerifySSL            *bool           `json:"verify_ssl,omitempty"`
	CustomHeaders        json.RawMessage `json:"custom_headers"`
	Description          string          `json:"description"`
	Environment          string          `json:"environment"`
	Region               string          `json:"region"`
}

// --- Snapshot Configs ---

type SnapshotConfig struct {
	ID               uuid.UUID       `json:"id"`
	OrgID            uuid.UUID       `json:"org_id"`
	SourceID         uuid.UUID       `json:"source_id"`
	SoftwareID       *uuid.UUID      `json:"software_id,omitempty"`
	Name             string          `json:"name"`
	SnapshotType     string          `json:"snapshot_type"`
	QueryTemplate    string          `json:"query_template"`
	TimeRangeSeconds int             `json:"time_range_seconds"`
	Parameters       json.RawMessage `json:"parameters"`
	Enabled          bool            `json:"enabled"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type CreateSnapshotConfigRequest struct {
	SourceID         uuid.UUID       `json:"source_id" binding:"required"`
	SoftwareID       *uuid.UUID      `json:"software_id,omitempty"`
	Name             string          `json:"name" binding:"required"`
	SnapshotType     string          `json:"snapshot_type" binding:"required"`
	QueryTemplate    string          `json:"query_template"`
	TimeRangeSeconds int             `json:"time_range_seconds"`
	Parameters       json.RawMessage `json:"parameters"`
}
