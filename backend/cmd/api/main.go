package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Rehzende/RootCauseway/backend/internal/auth"
	"github.com/Rehzende/RootCauseway/backend/internal/config"
	"github.com/Rehzende/RootCauseway/backend/internal/crypto"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/handlers"
	"github.com/Rehzende/RootCauseway/backend/internal/middleware"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
	"github.com/Rehzende/RootCauseway/backend/internal/ws"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("failed to parse redis URL: %v", err)
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	// WebSocket hub and Redis bridge
	wsHub := ws.NewHub()
	go wsHub.Run()

	redisBridge := ws.NewRedisBridge(rdb, wsHub)
	bridgeCtx, bridgeCancel := context.WithCancel(context.Background())
	defer bridgeCancel()
	go redisBridge.Start(bridgeCtx)

	// Handlers/services emit WS-bound events by publishing through
	// RedisEventPublisher (h.EventPublisher / s.publisher below) -- the
	// same dual-write (durable stream + pub/sub) used for alert.received --
	// rather than a separate Hub-only emitter. A parallel ws.EventEmitter
	// used to be instantiated here for this but was never actually called
	// from anywhere; removed rather than left as unused, misleading scaffolding.

	// Encryption-at-rest cipher for credential material (ROOTCAUSEWAY_ENCRYPTION_KEY).
	credCipher, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("failed to init credential encryption: %v", err)
	}

	softwareRepo := database.NewSoftwareRepository(pool)
	agentRepo := database.NewAgentRepository(pool)
	webhookRepo := database.NewWebhookRepository(pool)
	incidentRepo := database.NewIncidentRepository(pool)
	snapshotRepo := database.NewAlertSnapshotRepository(pool)
	publisher := database.NewRedisEventPublisher(rdb).WithStreamConfig(cfg.EventStreamName, cfg.EventStreamMaxLen)
	agentRunRepo := database.NewAgentRunRepository(pool)
	rciRepo := database.NewRCIRepository(pool)
	rcaRepo := database.NewRCARepository(pool)
	postmortemRepo := database.NewPostmortemRepository(pool)
	a2aAgentRepo := database.NewA2AAgentRepository(pool)
	a2aTaskRepo := database.NewA2ATaskRepository(pool)
	orchDecisionRepo := database.NewOrchestratorDecisionRepository(pool)
	skillRepo := database.NewSkillRepository(pool)
	agentSkillRepo := database.NewAgentSkillRepository(pool)
	credProviderRepo := database.NewCredentialProviderRepository(pool, credCipher)
	resCredRepo := database.NewResourceCredentialRepository(pool, credCipher)
	accessPolicyRepo := database.NewAccessPolicyRepository(pool)
	credLeaseRepo := database.NewCredentialLeaseRepository(pool)

	// Feature repositories
	feedbackRepo := database.NewFeedbackRepository(pool)
	knowledgeBaseRepo := database.NewKnowledgeBaseRepository(pool)
	similarIncidentRepo := database.NewSimilarIncidentRepository(pool)
	correlationRuleRepo := database.NewCorrelationRuleRepository(pool)
	alertGroupRepo := database.NewAlertGroupRepository(pool)
	notifChannelRepo := database.NewNotificationChannelRepository(pool)
	escalationPolicyRepo := database.NewEscalationPolicyRepository(pool)
	notifLogRepo := database.NewNotificationLogRepository(pool)
	runbookRepo := database.NewRunbookRepository(pool)
	runbookStepRepo := database.NewRunbookStepRepository(pool)
	runbookExecRepo := database.NewRunbookExecutionRepository(pool)
	changeEventRepo := database.NewChangeEventRepository(pool)
	analyticsRepo := database.NewAnalyticsRepository(pool)
	observabilitySourceRepo := database.NewObservabilitySourceRepository(pool)
	snapshotConfigRepo := database.NewSnapshotConfigRepository(pool)
	marketplaceAgentRepo := database.NewMarketplaceAgentRepository(pool)
	installedAgentRepo := database.NewInstalledAgentRepository(pool)
	warRoomRepo := database.NewWarRoomRepository(pool)
	sloRepo := database.NewSLORepository(pool)
	notifInteractionRepo := database.NewNotificationInteractionRepository(pool)

	// Auth repositories
	roleRepo := database.NewRoleRepository(pool)
	permissionRepo := database.NewPermissionRepository(pool)
	rolePermRepo := database.NewRolePermissionRepository(pool)
	userRoleRepo := database.NewUserRoleRepository(pool)
	ssoProviderRepo := database.NewSSOProviderRepository(pool)
	apiKeyRepo := database.NewAPIKeyRepository(pool)
	auditLogRepo := database.NewAuditLogRepository(pool)
	sessionRepo := database.NewSessionRepository(pool)
	userRepo := database.NewUserRepository(pool)

	// Auth components
	oidcAuth := auth.NewOIDCAuthenticator(ssoProviderRepo, userRepo, sessionRepo, userRoleRepo, roleRepo)
	rbacEnforcer := auth.NewRBACEnforcer(roleRepo, permissionRepo, userRoleRepo)
	apiKeyAuth := auth.NewAPIKeyAuthenticator(apiKeyRepo, userRoleRepo, permissionRepo)

	// Auth services
	roleSvc := services.NewRoleService(roleRepo, rolePermRepo, userRoleRepo, permissionRepo)
	ssoProviderSvc := services.NewSSOProviderService(ssoProviderRepo)
	apiKeySvc := services.NewAPIKeyService(apiKeyAuth, apiKeyRepo)
	auditSvc := services.NewAuditService(auditLogRepo)
	userSvc := services.NewUserService(userRepo, userRoleRepo)

	softwareSvc := services.NewSoftwareService(softwareRepo)
	agentSvc := services.NewAgentService(agentRepo)
	webhookSvc := services.NewWebhookService(webhookRepo)
	incidentSvc := services.NewIncidentService(incidentRepo, snapshotRepo)
	ingestionSvc := services.NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)
	ingestionSvc.SetSoftwareRepo(softwareRepo)
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

	// Feature services
	feedbackSvc := services.NewFeedbackService(feedbackRepo)
	knowledgeBaseSvc := services.NewKnowledgeBaseService(knowledgeBaseRepo)
	similarIncidentSvc := services.NewSimilarIncidentService(similarIncidentRepo)
	correlationRuleSvc := services.NewCorrelationRuleService(correlationRuleRepo)
	alertGroupSvc := services.NewAlertGroupService(alertGroupRepo)
	notifChannelSvc := services.NewNotificationChannelService(notifChannelRepo)
	escalationPolicySvc := services.NewEscalationPolicyService(escalationPolicyRepo)
	notifLogSvc := services.NewNotificationLogService(notifLogRepo)
	runbookSvc := services.NewRunbookService(runbookRepo)
	runbookStepSvc := services.NewRunbookStepService(runbookStepRepo)
	runbookExecSvc := services.NewRunbookExecutionService(runbookExecRepo)
	changeEventSvc := services.NewChangeEventService(changeEventRepo)
	analyticsSvc := services.NewAnalyticsService(analyticsRepo)
	observabilitySourceSvc := services.NewObservabilitySourceService(observabilitySourceRepo)
	snapshotConfigSvc := services.NewSnapshotConfigService(snapshotConfigRepo)
	sloSvc := services.NewSLOService(sloRepo)
	marketplaceSvc := services.NewMarketplaceService(marketplaceAgentRepo, installedAgentRepo, a2aAgentRepo)

	// Pipeline HITL gate (human-in-the-loop approval before postmortem) --
	// also owns org-level LLM and Teams integration settings, see
	// PgPipelineGateRepository's doc comment. Constructed before War Room
	// below since the Teams client resolver reads from it.
	pipelineGateRepo := database.NewPipelineGateRepository(pool)
	pgh := handlers.NewPipelineGateHandler(pipelineGateRepo, publisher, credProviderSvc)

	// War Room (Microsoft Teams meetings). The Teams client is resolved
	// per-org, per-call from each org's own settings (Integrations settings
	// UI, PgPipelineGateRepository.GetOrgTeamsSettings) rather than a single
	// client fixed at boot from TEAMS_* env vars -- real Graph API when an
	// org has all 4 fields configured, mock when WARROOM_MOCK_MODE=true,
	// otherwise a Noop client that errors clearly.
	teamsMockMode := strings.EqualFold(os.Getenv("WARROOM_MOCK_MODE"), "true")
	teamsResolver := services.NewTeamsClientResolver(pipelineGateRepo, pipelineGateRepo, teamsMockMode)
	warRoomSvc := services.NewWarRoomService(warRoomRepo, teamsResolver, incidentSvc)
	warRoomSvc.SetIncidentEventAdder(incidentSvc)
	warRoomSvc.SetEventPublisher(publisher)
	warRoomSvc.SetSoftwareReader(softwareSvc)

	// Teams delegated OAuth connect flow (see migration 027_teams_oauth):
	// a service/bot Microsoft account authorizes RootCauseway once via a normal
	// browser consent screen, replacing the old app-only auth path that
	// required a tenant admin to grant a Microsoft Application Access
	// Policy via PowerShell. teamsOAuthRedirectURI must be the exact,
	// absolute callback URL registered on every org's own Azure AD app
	// registration.
	teamsOAuthRedirectURI := strings.TrimRight(os.Getenv("ROOTCAUSEWAY_PUBLIC_API_URL"), "/") + "/api/v1/integrations/teams/oauth/callback"
	teamsFrontendBaseURL := strings.TrimRight(os.Getenv("ROOTCAUSEWAY_PUBLIC_APP_URL"), "/")
	teamsOAuthSvc := services.NewTeamsOAuthService(pipelineGateRepo, teamsOAuthRedirectURI)
	toh := handlers.NewTeamsOAuthHandler(teamsOAuthSvc, teamsFrontendBaseURL)

	quarantineRepo := database.NewAlertQuarantineRepository(pool)
	ingestionSvc.SetQuarantineRepo(quarantineRepo)

	// Postmortem export (Markdown/PDF).
	exportH := handlers.NewExportHandler(postmortemSvc, incidentSvc)

	// Data retention & archival (evidence / closed incidents / agent runs).
	retentionRepo := database.NewRetentionRepository(pool)
	retentionSvc := services.NewRetentionService(retentionRepo)
	rh := handlers.NewRetentionHandler(retentionSvc)

	// Bidirectional Slack/Teams notifications: acknowledge/resolve/view_rca
	// from inside the chat message. Read-only against incidentSvc/rcaSvc.
	notifInteractionSvc := services.NewNotificationInteractionService(incidentSvc, incidentSvc, rcaSvc, notifChannelRepo, notifInteractionRepo)
	nih := &handlers.NotificationInteractiveHandler{Interactions: notifInteractionSvc}

	h := handlers.NewHandler(softwareSvc, agentSvc, webhookSvc, incidentSvc, ingestionSvc, agentRunSvc, rciSvc, rcaSvc, postmortemSvc, a2aAgentSvc, a2aTaskSvc, orchDecisionSvc, skillSvc, agentSkillSvc, credProviderSvc, resCredSvc, accessPolicySvc, credLeaseSvc)
	h.EventPublisher = publisher
	h.QuarantineRepo = quarantineRepo

	ah := &handlers.AuthHandler{
		RoleSvc:        roleSvc,
		SSOProviderSvc: ssoProviderSvc,
		APIKeySvc:      apiKeySvc,
		AuditSvc:       auditSvc,
		UserSvc:        userSvc,
		OIDCAuth:       oidcAuth,
		RBAC:           rbacEnforcer,
		JWTSecret:      cfg.JWTSecret,
		UserRepo:       userRepo,
	}

	fh := &handlers.FeaturesHandler{
		Feedback:         feedbackSvc,
		KnowledgeBase:    knowledgeBaseSvc,
		Incidents:        incidentSvc,
		SimilarIncidents: similarIncidentSvc,
		CorrelationRules: correlationRuleSvc,
		AlertGroups:      alertGroupSvc,
		NotifChannels:    notifChannelSvc,
		EscalationPols:   escalationPolicySvc,
		NotifLog:         notifLogSvc,
		Runbooks:         runbookSvc,
		RunbookSteps:     runbookStepSvc,
		RunbookExecs:     runbookExecSvc,
		ChangeEvents:     changeEventSvc,
		Analytics:        analyticsSvc,
		EventPublisher:   publisher,
	}

	oh := &handlers.ObservabilityHandler{
		Sources:   observabilitySourceSvc,
		Snapshots: snapshotConfigSvc,
	}

	mh := &handlers.MarketplaceHandler{
		Marketplace: marketplaceSvc,
	}

	wrh := &handlers.WarRoomHandler{
		WarRooms: warRoomSvc,
	}

	ceh := &handlers.CorrelationExtraHandler{
		Software:     softwareSvc,
		IncidentRepo: incidentRepo,
	}

	sloh := handlers.NewSLOHandler(sloSvc)

	swSummaryH := &handlers.SoftwareSummaryHandler{
		Software:   softwareSvc,
		SLO:        sloRepo,
		Escalation: escalationPolicyRepo,
		Incidents:  incidentRepo,
	}

	r := gin.New()
	r.MaxMultipartMemory = 10 << 20 // 10MB

	// Middleware stack (order matters)
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.StructuredLogger())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Metrics())

	// Rate limiting deliberately does NOT get one blanket r.Use() here.
	// RateLimiter picks authenticatedRPM vs publicRPM by checking whether
	// "user_id" is already set in the gin context. A single global
	// r.Use(RateLimiter(...)) at this point in the chain (as this used to
	// be) runs before UnifiedAuthMiddleware ever gets a chance to set that
	// key -- for EVERY request, including ones headed to `protected`
	// routes -- so every authenticated user silently got the low
	// publicRPM=20 bucket instead of authenticatedRPM=100, and that one
	// 20rpm bucket ended up shared across every real browser besides: k3s's
	// default Traefik Service (externalTrafficPolicy: Cluster) SNATs
	// external traffic to the node's cni bridge IP before Gin ever sees
	// it, so every distinct client looked like the same IP too. Fixed the
	// network half by patching that Service to externalTrafficPolicy:
	// Local (klipper-lb then preserves the real source IP). This fixes the
	// other half: health/metrics/ws are infra endpoints (probes, Prometheus
	// scrapes, the dashboard's live socket) and are never rate limited at
	// all now; the small set of real pre-auth endpoints below get the
	// public tier explicitly; `protected` gets the authenticated tier
	// after UnifiedAuthMiddleware has actually run. Don't reintroduce a
	// single global call here -- it silently undoes this fix.
	publicRateLimiter := middleware.RateLimiter(100, 20)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://192.168.68.104:3000", "https://nonflexibly-advertizable-carlena.ngrok-free.dev"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Infra endpoints: never rate limited -- a slow/busy backend is exactly
	// when probes and the live dashboard socket matter most.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// WebSocket endpoint
	r.GET("/ws", handlers.HandleWebSocket(wsHub, cfg.JWTSecret))

	// Swagger / OpenAPI docs (public)
	r.GET("/api/docs", handlers.ServeSwaggerUI())
	r.GET("/api/docs/openapi.yaml", handlers.ServeOpenAPISpec("contracts/openapi/rootcauseway-api.yaml"))

	api := r.Group("/api/v1")

	// Genuinely public, pre-auth endpoints: rate limited at the public
	// tier (see publicRateLimiter comment above) since "user_id" is never
	// going to be set for any of these.
	api.POST("/auth/login", publicRateLimiter, ah.Login)
	api.GET("/auth/sso/:provider/login", publicRateLimiter, ah.SSOLogin)
	api.GET("/auth/sso/:provider/callback", publicRateLimiter, ah.SSOCallback)

	// Teams OAuth callback -- Microsoft's redirect lands here with no RootCauseway
	// JWT, same reasoning as the SSO callback above being public.
	api.GET("/integrations/teams/oauth/callback", publicRateLimiter, toh.Callback)

	// Public ingestion endpoint
	api.POST("/ingest/:token", publicRateLimiter, h.IngestAlert)

	// Public Slack/Teams interactive webhooks (bidirectional notifications).
	// Unauthenticated like /ingest/:token above -- these are called by
	// Slack/Teams directly, not by an RootCauseway user, so authenticity is
	// established per-request via signature/token verification against the
	// notification channel's own secret (see notification_interactive_handlers.go).
	api.POST("/webhooks/slack/interactive", publicRateLimiter, nih.SlackInteractive)
	api.POST("/webhooks/teams/interactive", publicRateLimiter, nih.TeamsInteractive)

	// Internal endpoints for agent-service (no JWT required, restricted by network)
	internal := api.Group("/internal")
	internal.Use(func(c *gin.Context) {
		orgIDStr := c.GetHeader("X-Org-ID")
		if orgIDStr != "" {
			if orgID, err := uuid.Parse(orgIDStr); err == nil {
				c.Set("org_id", orgID)
			}
		}
		c.Next()
	})
	internal.GET("/agents", h.ListAgents)
	internal.GET("/incidents/:id", h.GetIncident)
	internal.PATCH("/incidents/:id", h.UpdateIncident)
	internal.POST("/incidents/:id/events", h.AddIncidentEvent)
	internal.POST("/incidents/:id/evidence", h.AddIncidentEvidence)
	internal.POST("/incidents/:id/runs", h.CreateAgentRun)
	internal.PATCH("/incidents/:id/runs/:runId", h.UpdateAgentRun)
	internal.GET("/incidents/:id/rci", h.GetRCI)
	internal.POST("/incidents/:id/rci", h.CreateRCI)
	internal.GET("/incidents/:id/rca", h.GetRCA)
	internal.POST("/incidents/:id/rca", h.CreateRCA)
	internal.POST("/incidents/:id/postmortem", h.CreatePostmortem)
	internal.GET("/a2a/agents", h.ListA2AAgents)
	internal.GET("/a2a/agents/:id/card", h.GetA2AAgentCard)
	internal.POST("/incidents/:id/a2a/tasks", h.CreateA2ATask)
	internal.PATCH("/incidents/:id/a2a/tasks/:taskId", h.UpdateA2ATask)
	internal.POST("/incidents/:id/orchestrator/decisions", h.CreateOrchestratorDecision)
	internal.GET("/skills", h.ListSkills)
	internal.POST("/credentials/lease", h.RequestLease)
	internal.POST("/credentials/lease/:id/revoke", h.RevokeLease)
	internal.GET("/software/:id", h.GetSoftware)
	internal.GET("/software/:id/credentials", h.ListResourceCredentials)
	internal.POST("/access-policies/evaluate", h.EvaluatePolicies)
	internal.POST("/knowledge-base/search", fh.SearchKnowledgeBase)
	// GET variant: agent-service's search_knowledge_base sends
	// software_id/query/limit as query params, not a JSON body -- same
	// handler now (SearchKnowledgeBase uses ShouldBind), just registered
	// under the method agent-service actually calls.
	internal.GET("/knowledge-base/search", fh.SearchKnowledgeBase)
	internal.POST("/knowledge-base", fh.CreateKnowledgeBase)
	internal.POST("/knowledge-base/:id/increment-references", fh.IncrementKnowledgeBaseReferences)
	internal.GET("/correlation-rules", fh.ListCorrelationRules)
	internal.GET("/notification-channels", fh.ListNotificationChannels)
	internal.POST("/incidents/:id/feedback", fh.CreateFeedback)
	internal.POST("/change-events", fh.CreateChangeEvent)
	internal.POST("/incidents/:id/similar", fh.CreateSimilarIncident)
	internal.POST("/correlation/check", ceh.CorrelationCheck)
	internal.GET("/software/:id/observability", oh.ListSoftwareObservability)
	internal.GET("/observability/sources/:id", oh.GetSource)
	internal.GET("/warroom/:meetingId", wrh.GetWarRoomByID)
	internal.PATCH("/warroom/:meetingId/summary", wrh.AttachWarRoomSummary)
	internal.GET("/software/:id/dependency-graph", ceh.GetSoftwareDependencyGraph)
	internal.GET("/incidents/open-by-software", ceh.ListOpenIncidentsBySoftware)
	internal.GET("/incidents/by-fingerprint", ceh.FindIncidentByFingerprint)

	// Pipeline HITL gate (agent-service reads/writes gate state; no JWT).
	internal.GET("/organizations/:id/settings", pgh.GetOrgSettingsInternal)
	internal.POST("/incidents/:id/awaiting-approval", pgh.MarkAwaitingApprovalInternal)

	// Runbook automation (agent-service's RunbookExecutor drives "automated"
	// steps on the runbook.execution.started event; see ExecuteRunbook
	// above, which already published that event with nothing ever
	// consuming it -- these routes were the other missing half).
	internal.GET("/runbooks", fh.ListRunbooks)
	internal.GET("/runbooks/:id", fh.GetRunbook)
	internal.GET("/runbooks/:id/steps", fh.ListRunbookSteps)
	internal.GET("/runbook-executions/:execId", fh.GetRunbookExecution)
	internal.PATCH("/runbook-executions/:execId", fh.UpdateRunbookExecution)
	internal.POST("/runbook-executions/:execId/steps/:stepId/complete", fh.CompleteExecutionStep)

	protected := api.Group("")
	protected.Use(middleware.UnifiedAuthMiddleware(cfg.JWTSecret, apiKeyAuth, rbacEnforcer))
	// Registered AFTER auth on purpose: RateLimiter needs "user_id" already
	// set in context to grant the higher authenticatedRPM tier instead of
	// silently falling back to publicRPM -- see the comment on
	// publicRateLimiter above for the bug this fixes.
	protected.Use(middleware.RateLimiter(100, 20))
	protected.Use(middleware.AuditMiddleware(auditLogRepo))

	// Auth endpoints (protected)
	protected.POST("/auth/logout", ah.Logout)
	protected.GET("/auth/me", ah.Me)
	protected.POST("/auth/api-keys", ah.CreateAPIKey)
	protected.GET("/auth/api-keys", ah.ListAPIKeys)
	protected.DELETE("/auth/api-keys/:id", ah.RevokeAPIKey)

	// Users
	protected.GET("/users", ah.ListUsers)
	protected.POST("/users", ah.CreateUser)
	protected.GET("/users/:id", ah.GetUser)
	protected.PUT("/users/:id", ah.UpdateUser)
	protected.DELETE("/users/:id", ah.DeleteUser)
	protected.POST("/users/:id/roles", ah.AssignRole)
	protected.DELETE("/users/:id/roles/:roleId", ah.UnassignRole)

	// Roles
	protected.GET("/permissions", ah.ListPermissions)
	protected.GET("/roles", ah.ListRoles)
	protected.POST("/roles", ah.CreateRole)
	protected.GET("/roles/:id", ah.GetRole)
	protected.PUT("/roles/:id", ah.UpdateRole)
	protected.DELETE("/roles/:id", ah.DeleteRole)
	protected.POST("/roles/:id/permissions", ah.GrantPermission)
	protected.DELETE("/roles/:id/permissions/:permId", ah.RevokePermission)

	// SSO Providers
	protected.GET("/sso-providers", ah.ListSSOProviders)
	protected.POST("/sso-providers", ah.CreateSSOProvider)
	protected.PUT("/sso-providers/:id", ah.UpdateSSOProvider)
	protected.DELETE("/sso-providers/:id", ah.DeleteSSOProvider)

	// Audit Log
	protected.GET("/audit-log", ah.ListAuditLog)

	// Software Catalog
	protected.GET("/software", h.ListSoftware)
	protected.POST("/software", h.CreateSoftware)
	protected.GET("/software/:id", h.GetSoftware)
	protected.PUT("/software/:id", h.UpdateSoftware)
	protected.DELETE("/software/:id", h.DeleteSoftware)
	protected.GET("/software/:id/summary", swSummaryH.GetSoftwareSummary)

	// Agents
	protected.GET("/agents", h.ListAgents)
	protected.POST("/agents", h.CreateAgent)
	protected.GET("/agents/:id", h.GetAgent)
	protected.PUT("/agents/:id", h.UpdateAgent)
	protected.DELETE("/agents/:id", h.DeleteAgent)

	// Webhooks
	protected.GET("/webhooks", h.ListWebhooks)
	protected.POST("/webhooks", h.CreateWebhook)
	protected.GET("/webhooks/:id", h.GetWebhook)
	protected.DELETE("/webhooks/:id", h.DeleteWebhook)

	// Onboarding
	protected.GET("/onboarding/status", h.GetOnboardingStatus)

	// Quarantine
	protected.GET("/quarantine", h.ListQuarantine)
	protected.POST("/quarantine/:id/resolve", h.ResolveQuarantine)

	// Incidents
	protected.GET("/incidents", h.ListIncidents)
	protected.GET("/incidents/:id", h.GetIncident)
	protected.PATCH("/incidents/:id", h.UpdateIncident)
	protected.DELETE("/incidents/:id", h.DeleteIncident)
	protected.POST("/incidents/:id/events", h.AddIncidentEvent)
	protected.POST("/incidents/:id/evidence", h.AddIncidentEvidence)

	// Incident Cockpit
	protected.GET("/incidents/:id/full", h.GetIncidentFull)
	protected.GET("/incidents/:id/dag", h.GetIncidentDAG)
	protected.GET("/incidents/:id/runs", h.ListAgentRuns)
	protected.GET("/incidents/:id/runs/:runId", h.GetAgentRun)
	protected.POST("/incidents/:id/runs/:runId/rerun", h.RerunAgentRun)
	protected.POST("/incidents/:id/rci", h.CreateRCI)
	protected.GET("/incidents/:id/rci", h.GetRCI)
	protected.PATCH("/incidents/:id/rci", h.UpdateRCI)
	protected.POST("/incidents/:id/rca", h.CreateRCA)
	protected.GET("/incidents/:id/rca", h.GetRCA)
	protected.PATCH("/incidents/:id/rca", h.UpdateRCA)
	protected.POST("/incidents/:id/postmortem", h.CreatePostmortem)
	protected.GET("/incidents/:id/postmortem", h.GetPostmortem)
	protected.PATCH("/incidents/:id/postmortem", h.UpdatePostmortem)
	protected.GET("/incidents/:id/postmortem/export", exportH.ExportPostmortem)
	protected.POST("/incidents/:id/evidence/upload", h.UploadEvidence)

	// A2A Agents
	protected.GET("/a2a/agents", h.ListA2AAgents)
	protected.POST("/a2a/agents", h.CreateA2AAgent)
	protected.POST("/a2a/agents/health-check-all", h.HealthCheckAllA2AAgents)
	protected.GET("/a2a/agents/:id", h.GetA2AAgent)
	protected.PUT("/a2a/agents/:id", h.UpdateA2AAgent)
	protected.DELETE("/a2a/agents/:id", h.DeleteA2AAgent)
	protected.GET("/a2a/agents/:id/card", h.GetA2AAgentCard)
	protected.POST("/a2a/agents/:id/health-check", h.HealthCheckA2AAgent)

	// A2A Tasks
	protected.GET("/incidents/:id/a2a/tasks", h.ListA2ATasks)
	protected.POST("/incidents/:id/a2a/tasks", h.CreateA2ATask)
	protected.GET("/incidents/:id/a2a/tasks/:taskId", h.GetA2ATask)
	protected.PATCH("/incidents/:id/a2a/tasks/:taskId", h.UpdateA2ATask)

	// Orchestrator Decisions
	protected.GET("/incidents/:id/orchestrator/decisions", h.ListOrchestratorDecisions)

	// Skills
	protected.GET("/skills", h.ListSkills)
	protected.POST("/skills", h.CreateSkill)
	protected.GET("/skills/:id", h.GetSkill)
	protected.PUT("/skills/:id", h.UpdateSkill)
	protected.DELETE("/skills/:id", h.DeleteSkill)
	protected.GET("/skills/:id/agents", h.ListSkillAgents)

	// Agent Skills
	protected.GET("/a2a/agents/:id/skills", h.ListAgentSkills)
	protected.POST("/a2a/agents/:id/skills", h.LinkSkillToAgent)
	protected.DELETE("/a2a/agents/:id/skills/:skillId", h.UnlinkSkillFromAgent)

	// Credential Providers
	protected.GET("/credentials/providers", h.ListProviders)
	protected.POST("/credentials/providers", h.CreateProvider)
	protected.GET("/credentials/providers/:id", h.GetProvider)
	protected.PUT("/credentials/providers/:id", h.UpdateProvider)
	protected.DELETE("/credentials/providers/:id", h.DeleteProvider)

	// Resource Credentials
	protected.GET("/software/:id/credentials", h.ListResourceCredentials)
	protected.POST("/software/:id/credentials", h.CreateResourceCredential)
	protected.GET("/software/:id/credentials/:credId", h.GetResourceCredential)
	protected.PUT("/software/:id/credentials/:credId", h.UpdateResourceCredential)
	protected.DELETE("/software/:id/credentials/:credId", h.DeleteResourceCredential)

	// Access Policies
	protected.GET("/access-policies", h.ListPolicies)
	protected.POST("/access-policies", h.CreatePolicy)
	protected.GET("/access-policies/:id", h.GetPolicy)
	protected.PUT("/access-policies/:id", h.UpdatePolicy)
	protected.DELETE("/access-policies/:id", h.DeletePolicy)

	// Feedback
	protected.GET("/incidents/:id/feedback", fh.ListFeedback)
	protected.POST("/incidents/:id/feedback", fh.CreateFeedback)

	// Knowledge Base
	protected.GET("/knowledge-base", fh.ListKnowledgeBase)
	protected.POST("/knowledge-base", fh.CreateKnowledgeBase)
	// /knowledge-base/search must be registered before /knowledge-base/:id
	// or gin's router would treat "search" as an :id value instead.
	protected.GET("/knowledge-base/search", fh.SearchKnowledgeBase)
	protected.POST("/knowledge-base/search", fh.SearchKnowledgeBase)
	protected.GET("/knowledge-base/:id", fh.GetKnowledgeBase)
	protected.PUT("/knowledge-base/:id", fh.UpdateKnowledgeBase)

	// Similar Incidents
	protected.GET("/incidents/:id/similar", fh.ListSimilarIncidents)

	// Correlation Rules
	protected.GET("/correlation-rules", fh.ListCorrelationRules)
	protected.POST("/correlation-rules", fh.CreateCorrelationRule)
	protected.GET("/correlation-rules/:id", fh.GetCorrelationRule)
	protected.PUT("/correlation-rules/:id", fh.UpdateCorrelationRule)
	protected.DELETE("/correlation-rules/:id", fh.DeleteCorrelationRule)

	// Alert Groups
	protected.GET("/incidents/:id/alert-groups", fh.ListAlertGroups)

	// Notification Channels
	protected.GET("/notification-channels", fh.ListNotificationChannels)
	protected.POST("/notification-channels", fh.CreateNotificationChannel)
	protected.GET("/notification-channels/:id", fh.GetNotificationChannel)
	protected.PUT("/notification-channels/:id", fh.UpdateNotificationChannel)
	protected.DELETE("/notification-channels/:id", fh.DeleteNotificationChannel)

	// Escalation Policies
	protected.GET("/escalation-policies", fh.ListEscalationPolicies)
	protected.POST("/escalation-policies", fh.CreateEscalationPolicy)
	protected.GET("/escalation-policies/:id", fh.GetEscalationPolicy)
	protected.PUT("/escalation-policies/:id", fh.UpdateEscalationPolicy)
	protected.DELETE("/escalation-policies/:id", fh.DeleteEscalationPolicy)

	// Notification Log
	protected.GET("/notifications/logs", fh.ListNotificationLogsGlobal)
	protected.GET("/incidents/:id/notifications", fh.ListNotificationLog)

	// Runbooks
	protected.GET("/runbooks", fh.ListRunbooks)
	protected.POST("/runbooks", fh.CreateRunbook)
	protected.GET("/runbooks/:id", fh.GetRunbook)
	protected.PUT("/runbooks/:id", fh.UpdateRunbook)
	protected.DELETE("/runbooks/:id", fh.DeleteRunbook)
	protected.GET("/runbooks/:id/steps", fh.ListRunbookSteps)
	protected.POST("/runbooks/:id/steps", fh.CreateRunbookStep)
	protected.PUT("/runbooks/:id/steps/:stepId", fh.UpdateRunbookStep)
	protected.DELETE("/runbooks/:id/steps/:stepId", fh.DeleteRunbookStep)
	protected.POST("/runbooks/:id/steps/reorder", fh.ReorderRunbookSteps)
	protected.POST("/runbooks/:id/execute", fh.ExecuteRunbook)
	protected.GET("/runbooks/:id/executions", fh.ListRunbookExecutionsByRunbook)
	protected.GET("/runbook-executions/:execId", fh.GetRunbookExecution)
	protected.PATCH("/runbook-executions/:execId", fh.UpdateRunbookExecution)
	protected.POST("/runbook-executions/:execId/steps/:stepId/complete", fh.CompleteExecutionStep)
	protected.GET("/incidents/:id/runbook-executions", fh.ListIncidentRunbookExecutions)

	// Change Events
	protected.GET("/change-events", fh.ListChangeEvents)
	protected.POST("/change-events", fh.CreateChangeEvent)
	protected.GET("/software/:id/changes", fh.ListSoftwareChanges)

	// Analytics
	protected.GET("/analytics/mttr", fh.GetMTTR)
	protected.GET("/analytics/trends", fh.GetIncidentTrends)
	protected.GET("/analytics/agent-effectiveness", fh.GetAgentEffectiveness)
	protected.GET("/analytics/cost-by-model", fh.GetCostByModel)
	protected.GET("/analytics/cost-by-incident", fh.GetCostByIncident)

	// SLO / Error Budget Tracking
	protected.GET("/slo-definitions", sloh.ListSLODefinitions)
	protected.POST("/slo-definitions", sloh.CreateSLODefinition)
	protected.GET("/slo-definitions/:id", sloh.GetSLODefinition)
	protected.PUT("/slo-definitions/:id", sloh.UpdateSLODefinition)
	protected.DELETE("/slo-definitions/:id", sloh.DeleteSLODefinition)
	protected.GET("/slo-definitions/:id/status", sloh.GetSLOStatus)
	protected.GET("/software/:id/slo-status", sloh.GetSoftwareSLOStatus)

	// Observability Sources
	protected.GET("/observability/sources", oh.ListSources)
	protected.POST("/observability/sources", oh.CreateSource)
	protected.GET("/observability/sources/:id", oh.GetSource)
	protected.PUT("/observability/sources/:id", oh.UpdateSource)
	protected.DELETE("/observability/sources/:id", oh.DeleteSource)
	protected.POST("/observability/sources/:id/health", oh.CheckSourceHealth)
	protected.GET("/observability/sources/:id/snapshots", oh.ListSnapshotConfigs)
	protected.POST("/observability/sources/:id/snapshots", oh.CreateSnapshotConfig)
	protected.GET("/observability/snapshots/:id", oh.GetSnapshotConfig)
	protected.PUT("/observability/snapshots/:id", oh.UpdateSnapshotConfig)
	protected.DELETE("/observability/snapshots/:id", oh.DeleteSnapshotConfig)
	protected.GET("/software/:id/observability", oh.ListSoftwareObservability)

	// Marketplace
	protected.GET("/marketplace", mh.ListMarketplaceAgents)
	protected.GET("/marketplace/installed", mh.ListInstalledAgents)
	protected.GET("/marketplace/:slug", mh.GetMarketplaceAgent)
	protected.POST("/marketplace/:slug/install", mh.InstallAgent)
	protected.DELETE("/marketplace/installed/:id", mh.UninstallAgent)

	// War Room
	protected.POST("/incidents/:id/warroom", wrh.CreateWarRoom)
	protected.GET("/incidents/:id/warroom", wrh.GetWarRoom)
	protected.POST("/warroom/:meetingId/end", wrh.EndWarRoom)

	// Credential Leases
	protected.GET("/credentials/leases", h.ListLeases)
	protected.GET("/credentials/leases/active", h.ListActiveLeases)
	protected.POST("/credentials/leases/request", h.RequestLease)
	protected.POST("/credentials/leases/:id/revoke", h.RevokeLease)

	// Pipeline HITL Gate
	protected.POST("/incidents/:id/approve-stage", pgh.ApproveStage)
	protected.GET("/organizations/:id/settings", pgh.GetOrgSettings)
	protected.PATCH("/organizations/:id/settings", pgh.UpdateOrgSettings)
	protected.POST("/organizations/:id/integrations/teams/oauth/authorize", toh.Authorize)
	protected.POST("/organizations/:id/integrations/teams/oauth/disconnect", toh.Disconnect)

	// Retention Policies (evidence / closed incidents / agent runs archival)
	protected.GET("/retention-policies", rh.ListRetentionPolicies)
	protected.POST("/retention-policies", rh.CreateRetentionPolicy)
	protected.PUT("/retention-policies/:id", rh.UpdateRetentionPolicy)
	protected.DELETE("/retention-policies/:id", rh.DeleteRetentionPolicy)
	protected.POST("/retention-policies/sweep", rh.TriggerSweep)

	// Seed default roles for all orgs (use nil UUID for global defaults)
	// In production, this would iterate orgs. For MVP, seed for a default org.
	go func() {
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer seedCancel()
		// Try to seed for the first org found
		var defaultOrgID uuid.UUID
		_ = pool.QueryRow(seedCtx, `SELECT id FROM organizations LIMIT 1`).Scan(&defaultOrgID)
		if defaultOrgID != uuid.Nil {
			if err := handlers.SeedDefaultRoles(seedCtx, roleRepo, permissionRepo, rolePermRepo, defaultOrgID); err != nil {
				log.Printf("Warning: failed to seed default roles: %v", err)
			}
		}
	}()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		log.Printf("RootCauseway Backend starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exited")
}
