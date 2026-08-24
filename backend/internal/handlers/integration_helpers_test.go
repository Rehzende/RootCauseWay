//go:build integration

package handlers

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/middleware"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-jwt-secret-for-integration-tests"

func getTestDBURL() string {
	url := os.Getenv("ROOTCAUSEWAY_TEST_DB_URL")
	if url == "" {
		url = "postgres://rootcauseway:rootcauseway_dev_password@localhost:5432/test_rootcauseway?sslmode=disable"
	}
	return url
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, getTestDBURL())
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func setupTestRouter(t *testing.T, pool *pgxpool.Pool) (*gin.Engine, *Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	softwareRepo := database.NewSoftwareRepository(pool)
	agentRepo := database.NewAgentRepository(pool)
	webhookRepo := database.NewWebhookRepository(pool)
	incidentRepo := database.NewIncidentRepository(pool)
	snapshotRepo := database.NewAlertSnapshotRepository(pool)
	agentRunRepo := database.NewAgentRunRepository(pool)
	rciRepo := database.NewRCIRepository(pool)
	rcaRepo := database.NewRCARepository(pool)
	postmortemRepo := database.NewPostmortemRepository(pool)
	a2aAgentRepo := database.NewA2AAgentRepository(pool)
	a2aTaskRepo := database.NewA2ATaskRepository(pool)
	orchDecisionRepo := database.NewOrchestratorDecisionRepository(pool)
	skillRepo := database.NewSkillRepository(pool)
	agentSkillRepo := database.NewAgentSkillRepository(pool)
	credProviderRepo := database.NewCredentialProviderRepository(pool)
	resCredRepo := database.NewResourceCredentialRepository(pool)
	accessPolicyRepo := database.NewAccessPolicyRepository(pool)
	credLeaseRepo := database.NewCredentialLeaseRepository(pool)

	// Stub publisher that does nothing
	publisher := &noopPublisher{}

	softwareSvc := services.NewSoftwareService(softwareRepo)
	agentSvc := services.NewAgentService(agentRepo)
	webhookSvc := services.NewWebhookService(webhookRepo)
	incidentSvc := services.NewIncidentService(incidentRepo, snapshotRepo)
	ingestionSvc := services.NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)
	agentRunSvc := services.NewAgentRunService(agentRunRepo)
	rciSvc := services.NewRCIService(rciRepo)
	rcaSvc := services.NewRCAService(rcaRepo, incidentRepo)
	postmortemSvc := services.NewPostmortemService(postmortemRepo)
	a2aAgentSvc := services.NewA2AAgentService(a2aAgentRepo)
	a2aTaskSvc := services.NewA2ATaskService(a2aTaskRepo)
	orchDecisionSvc := services.NewOrchestratorDecisionService(orchDecisionRepo)
	skillSvc := services.NewSkillService(skillRepo)
	agentSkillSvc := services.NewAgentSkillService(agentSkillRepo)
	credProviderSvc := services.NewCredentialProviderService(credProviderRepo)
	resCredSvc := services.NewResourceCredentialService(resCredRepo)
	accessPolicySvc := services.NewAccessPolicyService(accessPolicyRepo)
	credLeaseSvc := services.NewCredentialLeaseService(credLeaseRepo, accessPolicyRepo, resCredRepo, credProviderRepo)

	h := NewHandler(softwareSvc, agentSvc, webhookSvc, incidentSvc, ingestionSvc,
		agentRunSvc, rciSvc, rcaSvc, postmortemSvc, a2aAgentSvc, a2aTaskSvc,
		orchDecisionSvc, skillSvc, agentSkillSvc, credProviderSvc, resCredSvc,
		accessPolicySvc, credLeaseSvc)

	r := gin.New()

	// Public endpoints
	api := r.Group("/api/v1")
	api.POST("/ingest/:token", h.IngestAlert)

	// Protected endpoints
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(testJWTSecret))

	protected.GET("/software", h.ListSoftware)
	protected.POST("/software", h.CreateSoftware)
	protected.GET("/software/:id", h.GetSoftware)
	protected.PUT("/software/:id", h.UpdateSoftware)
	protected.DELETE("/software/:id", h.DeleteSoftware)

	protected.GET("/webhooks", h.ListWebhooks)
	protected.POST("/webhooks", h.CreateWebhook)
	protected.GET("/webhooks/:id", h.GetWebhook)
	protected.DELETE("/webhooks/:id", h.DeleteWebhook)

	protected.GET("/incidents", h.ListIncidents)
	protected.GET("/incidents/:id", h.GetIncident)
	protected.PATCH("/incidents/:id", h.UpdateIncident)
	protected.POST("/incidents/:id/events", h.AddIncidentEvent)
	protected.GET("/incidents/:id/full", h.GetIncidentFull)

	protected.GET("/a2a/agents", h.ListA2AAgents)
	protected.POST("/a2a/agents", h.CreateA2AAgent)
	protected.GET("/a2a/agents/:id", h.GetA2AAgent)
	protected.PUT("/a2a/agents/:id", h.UpdateA2AAgent)
	protected.DELETE("/a2a/agents/:id", h.DeleteA2AAgent)
	protected.GET("/a2a/agents/:id/card", h.GetA2AAgentCard)

	protected.GET("/skills", h.ListSkills)
	protected.POST("/skills", h.CreateSkill)
	protected.GET("/skills/:id", h.GetSkill)
	protected.PUT("/skills/:id", h.UpdateSkill)
	protected.DELETE("/skills/:id", h.DeleteSkill)

	protected.GET("/a2a/agents/:id/skills", h.ListAgentSkills)
	protected.POST("/a2a/agents/:id/skills", h.LinkSkillToAgent)
	protected.DELETE("/a2a/agents/:id/skills/:skillId", h.UnlinkSkillFromAgent)

	protected.GET("/credentials/providers", h.ListProviders)
	protected.POST("/credentials/providers", h.CreateProvider)
	protected.GET("/credentials/providers/:id", h.GetProvider)
	protected.PUT("/credentials/providers/:id", h.UpdateProvider)
	protected.DELETE("/credentials/providers/:id", h.DeleteProvider)

	protected.GET("/software/:id/credentials", h.ListResourceCredentials)
	protected.POST("/software/:id/credentials", h.CreateResourceCredential)

	protected.GET("/access-policies", h.ListPolicies)
	protected.POST("/access-policies", h.CreatePolicy)

	protected.POST("/credentials/leases/request", h.RequestLease)
	protected.POST("/credentials/leases/:id/revoke", h.RevokeLease)
	protected.GET("/credentials/leases/active", h.ListActiveLeases)

	return r, h
}

func createTestOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	orgID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`,
		orgID, "test-org-"+orgID.String()[:8], "test-org-"+orgID.String()[:8])
	if err != nil {
		t.Fatalf("failed to create test org: %v", err)
	}
	return orgID
}

func createTestUser(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) (uuid.UUID, string) {
	t.Helper()
	userID := uuid.New()
	hash, _ := bcrypt.GenerateFromPassword([]byte("testpassword123"), bcrypt.MinCost)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, org_id, name, email, password_hash, role) VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, orgID, "Test User", fmt.Sprintf("test-%s@example.com", userID.String()[:8]),
		string(hash), "admin")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	claims := middleware.Claims{
		UserID: userID,
		OrgID:  orgID,
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return userID, tokenStr
}

func cleanupTestData(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"credential_leases", "access_policies", "resource_credentials",
		"credential_providers", "agent_skill_links", "skills",
		"orchestrator_decisions", "a2a_tasks", "a2a_agents",
		"incident_postmortems", "incident_rcas", "incident_rcis",
		"agent_runs", "incident_evidence", "incident_events",
		"alert_snapshots", "incidents", "webhooks", "agents",
		"software_catalog", "users", "organizations",
	}
	for _, table := range tables {
		pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE org_id = $1", table), orgID)
	}
	// Tables without org_id
	pool.Exec(ctx, "DELETE FROM organizations WHERE id = $1", orgID)
}

// noopPublisher is a stub EventPublisher for integration tests.
type noopPublisher struct{}

func (n *noopPublisher) Publish(ctx context.Context, channel string, event models.EventEnvelope) error {
	return nil
}
