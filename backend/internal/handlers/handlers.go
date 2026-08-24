package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/middleware"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
)

// Service interfaces used by handlers

type SoftwareServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateSoftwareRequest) (*models.SoftwareEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SoftwareEntry, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateSoftwareRequest) (*models.SoftwareEntry, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AgentServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateAgentRequest) (*models.Agent, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.Agent, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateAgentRequest) (*models.Agent, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type WebhookServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateWebhookRequest) (*models.Webhook, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Webhook, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type IncidentServiceInterface interface {
	Create(ctx context.Context, incident *models.Incident) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error)
	List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateIncidentRequest) (*models.Incident, bool, error)
	AddEvent(ctx context.Context, incidentID uuid.UUID, actor string, req models.CreateEventRequest) (*models.IncidentEvent, error)
	AddEvidence(ctx context.Context, incidentID uuid.UUID, req models.CreateEvidenceRequest) (*models.IncidentEvidence, error)
	ListEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error)
}

type IngestionServiceInterface interface {
	IngestAlert(ctx context.Context, token string, rawPayload json.RawMessage) (*services.IngestionResult, error)
}

type AgentRunServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateAgentRunRequest) (*models.AgentRun, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AgentRun, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateAgentRunRequest) (*models.AgentRun, error)
	GetDAG(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error)
}

type RCIServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateRCIRequest) (*models.IncidentRCI, error)
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCI, error)
	Update(ctx context.Context, incidentID uuid.UUID, req models.CreateRCIRequest) (*models.IncidentRCI, error)
}

type RCAServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateRCARequest) (*models.IncidentRCA, error)
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error)
	Update(ctx context.Context, incidentID uuid.UUID, req models.CreateRCARequest) (*models.IncidentRCA, error)
}

type PostmortemServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreatePostmortemRequest) (*models.IncidentPostmortem, error)
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentPostmortem, error)
	Update(ctx context.Context, incidentID uuid.UUID, req models.CreatePostmortemRequest) (*models.IncidentPostmortem, error)
}

type A2AAgentServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateA2AAgentRequest) (*models.A2AAgent, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.A2AAgent, error)
	List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.A2AAgent, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateA2AAgentRequest) (*models.A2AAgent, error)
	Delete(ctx context.Context, id uuid.UUID) error
	HealthCheck(ctx context.Context, id uuid.UUID, status string) error
	GetBySkill(ctx context.Context, orgID uuid.UUID, skill string) ([]models.A2AAgent, error)
}

type A2ATaskServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateA2ATaskRequest) (*models.A2ATask, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.A2ATask, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.A2ATask, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateA2ATaskRequest) (*models.A2ATask, error)
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.A2ATask, error)
}

type OrchestratorDecisionServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateOrchestratorDecisionRequest) (*models.OrchestratorDecision, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.OrchestratorDecision, error)
}

type SkillServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateSkillRequest) (*models.Skill, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error)
	List(ctx context.Context, orgID uuid.UUID, category string, page, perPage int) ([]models.Skill, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateSkillRequest) (*models.Skill, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AgentSkillServiceInterface interface {
	Link(ctx context.Context, agentID uuid.UUID, req models.CreateAgentSkillLinkRequest) (*models.AgentSkillLink, error)
	Unlink(ctx context.Context, agentID, skillID uuid.UUID) error
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.AgentSkillLink, error)
	ListBySkill(ctx context.Context, skillID uuid.UUID) ([]models.AgentSkillLink, error)
}

type CredentialProviderServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateCredentialProviderRequest) (*models.CredentialProvider, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialProvider, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.CredentialProvider, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateCredentialProviderRequest) (*models.CredentialProvider, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ResourceCredentialServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateResourceCredentialRequest) (*models.ResourceCredential, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ResourceCredential, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ResourceCredential, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateResourceCredentialRequest) (*models.ResourceCredential, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AccessPolicyServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateAccessPolicyRequest) (*models.AccessPolicy, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AccessPolicy, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.AccessPolicy, int, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateAccessPolicyRequest) (*models.AccessPolicy, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Evaluate(ctx context.Context, orgID uuid.UUID, agentID, skillID uuid.UUID, resourceType string) ([]models.AccessPolicy, error)
}

type CredentialLeaseServiceInterface interface {
	RequestLease(ctx context.Context, orgID uuid.UUID, req models.RequestLeaseRequest) (*models.CredentialLease, error)
	RevokeLease(ctx context.Context, id uuid.UUID, revokedBy string) (*models.CredentialLease, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialLease, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.CredentialLease, error)
	ListActive(ctx context.Context, orgID uuid.UUID) ([]models.CredentialLease, error)
}

type EventPublisherInterface interface {
	Publish(ctx context.Context, channel string, event models.EventEnvelope) error
}

type Handler struct {
	Software              SoftwareServiceInterface
	Agents                AgentServiceInterface
	Webhooks              WebhookServiceInterface
	Incidents             IncidentServiceInterface
	Ingestion             IngestionServiceInterface
	AgentRuns             AgentRunServiceInterface
	RCI                   RCIServiceInterface
	RCA                   RCAServiceInterface
	Postmortem            PostmortemServiceInterface
	A2AAgents             A2AAgentServiceInterface
	A2ATasks              A2ATaskServiceInterface
	OrchestratorDecisions OrchestratorDecisionServiceInterface
	Skills                SkillServiceInterface
	AgentSkills           AgentSkillServiceInterface
	CredentialProviders   CredentialProviderServiceInterface
	ResourceCredentials   ResourceCredentialServiceInterface
	AccessPolicies        AccessPolicyServiceInterface
	CredentialLeases      CredentialLeaseServiceInterface
	EventPublisher        EventPublisherInterface
	QuarantineRepo        *database.PgAlertQuarantineRepository
}

func NewHandler(sw SoftwareServiceInterface, ag AgentServiceInterface, wh WebhookServiceInterface, inc IncidentServiceInterface, ing IngestionServiceInterface, ar AgentRunServiceInterface, rci RCIServiceInterface, rca RCAServiceInterface, pm PostmortemServiceInterface, a2aAgents A2AAgentServiceInterface, a2aTasks A2ATaskServiceInterface, orchDecisions OrchestratorDecisionServiceInterface, skills SkillServiceInterface, agentSkills AgentSkillServiceInterface, credProviders CredentialProviderServiceInterface, resCreds ResourceCredentialServiceInterface, accessPolicies AccessPolicyServiceInterface, credLeases CredentialLeaseServiceInterface) *Handler {
	return &Handler{Software: sw, Agents: ag, Webhooks: wh, Incidents: inc, Ingestion: ing, AgentRuns: ar, RCI: rci, RCA: rca, Postmortem: pm, A2AAgents: a2aAgents, A2ATasks: a2aTasks, OrchestratorDecisions: orchDecisions, Skills: skills, AgentSkills: agentSkills, CredentialProviders: credProviders, ResourceCredentials: resCreds, AccessPolicies: accessPolicies, CredentialLeases: credLeases}
}

func getOrgID(c *gin.Context) uuid.UUID {
	v, _ := c.Get("org_id")
	id, _ := v.(uuid.UUID)
	return id
}

func getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	return middleware.ValidatePagination(page, perPage)
}

// handleDBError maps database errors to appropriate HTTP responses.
// It never exposes raw DB error messages to the client.
func handleDBError(c *gin.Context, err error, entity string) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: entity + " not found"})
		return
	}
	slog.Error("database error",
		"request_id", c.GetString("request_id"),
		"entity", entity,
		"error", err.Error(),
	)
	c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
}
