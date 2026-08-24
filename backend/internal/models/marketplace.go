package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Agent Marketplace ---

type MarketplaceAgent struct {
	ID                  uuid.UUID       `json:"id"`
	Name                string          `json:"name"`
	Slug                string          `json:"slug"`
	Description         string          `json:"description"`
	LongDescription     string          `json:"long_description"`
	Author              string          `json:"author"`
	AuthorURL           string          `json:"author_url"`
	Version             string          `json:"version"`
	Category            string          `json:"category"`
	IconURL             string          `json:"icon_url"`
	DockerImage         string          `json:"docker_image"`
	AgentCard           json.RawMessage `json:"agent_card"`
	Skills              json.RawMessage `json:"skills"`
	RequiredCredentials json.RawMessage `json:"required_credentials"`
	ConfigSchema        json.RawMessage `json:"config_schema"`
	Readme              string          `json:"readme"`
	Downloads           int             `json:"downloads"`
	Rating              float64         `json:"rating"`
	Verified            bool            `json:"verified"`
	Published           bool            `json:"published"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type InstalledAgent struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	MarketplaceAgentID uuid.UUID       `json:"marketplace_agent_id"`
	A2AAgentID         *uuid.UUID      `json:"a2a_agent_id,omitempty"`
	Config             json.RawMessage `json:"config"`
	Version            string          `json:"version"`
	Status             string          `json:"status"`
	InstalledAt        time.Time       `json:"installed_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type InstallAgentRequest struct {
	MarketplaceAgentID uuid.UUID       `json:"marketplace_agent_id" binding:"required"`
	Config             json.RawMessage `json:"config"`
}
