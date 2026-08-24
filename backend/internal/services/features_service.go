package services

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/embeddings"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Repository Interfaces ---

type FeedbackRepository interface {
	Create(ctx context.Context, f *models.IncidentFeedback) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentFeedback, error)
}

type KnowledgeBaseRepository interface {
	Create(ctx context.Context, kb *models.KnowledgeBaseEntry) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.KnowledgeBaseEntry, error)
	List(ctx context.Context, orgID uuid.UUID, category string) ([]models.KnowledgeBaseEntry, error)
	Search(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, errorPattern string) ([]models.KnowledgeBaseEntry, error)
	Update(ctx context.Context, kb *models.KnowledgeBaseEntry) error
	IncrementReferences(ctx context.Context, id uuid.UUID) error
}

// KnowledgeBaseVectorRepository is the optional vector-search extension of
// KnowledgeBaseRepository (implemented by the pgvector-backed repository).
type KnowledgeBaseVectorRepository interface {
	UpdateEmbedding(ctx context.Context, id uuid.UUID, embedding []float32) error
	SearchByEmbedding(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, embedding []float32, minSimilarity float64, limit int) ([]models.KnowledgeBaseEntry, error)
}

type SimilarIncidentRepository interface {
	Create(ctx context.Context, s *models.SimilarIncident) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.SimilarIncident, error)
}

// IncidentVectorRepository provides pgvector operations over incidents.
type IncidentVectorRepository interface {
	UpdateEmbedding(ctx context.Context, incidentID uuid.UUID, embedding []float32) error
	FindSimilar(ctx context.Context, incidentID uuid.UUID, limit int) ([]models.SimilarIncidentMatch, error)
}

type CorrelationRuleRepository interface {
	Create(ctx context.Context, cr *models.CorrelationRule) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.CorrelationRule, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.CorrelationRule, error)
	Update(ctx context.Context, cr *models.CorrelationRule) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AlertGroupRepository interface {
	Create(ctx context.Context, ag *models.AlertGroup) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AlertGroup, error)
}

type NotificationChannelRepository interface {
	Create(ctx context.Context, nc *models.NotificationChannel) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationChannel, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.NotificationChannel, error)
	Update(ctx context.Context, nc *models.NotificationChannel) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type EscalationPolicyRepository interface {
	Create(ctx context.Context, ep *models.EscalationPolicy) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.EscalationPolicy, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.EscalationPolicy, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.EscalationPolicy, error)
	Update(ctx context.Context, ep *models.EscalationPolicy) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type NotificationLogRepository interface {
	Create(ctx context.Context, nl *models.NotificationLogEntry) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.NotificationLogEntry, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.NotificationLogEntry, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, errMsg string) error
}

type RunbookRepository interface {
	Create(ctx context.Context, rb *models.Runbook) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Runbook, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Runbook, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.Runbook, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.Runbook, error)
	Update(ctx context.Context, rb *models.Runbook) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type RunbookStepRepository interface {
	Create(ctx context.Context, s *models.RunbookStep) error
	ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookStep, error)
	Update(ctx context.Context, s *models.RunbookStep) error
	UpdateOrder(ctx context.Context, id uuid.UUID, order int) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type RunbookExecutionRepository interface {
	Create(ctx context.Context, re *models.RunbookExecution) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.RunbookExecution, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.RunbookExecution, error)
	ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookExecution, error)
	Update(ctx context.Context, re *models.RunbookExecution) error
}

type ChangeEventRepository interface {
	Create(ctx context.Context, ce *models.ChangeEvent) error
	ListBySoftware(ctx context.Context, softwareID uuid.UUID, since time.Time) ([]models.ChangeEvent, error)
	ListRecent(ctx context.Context, softwareID uuid.UUID, minutes int) ([]models.ChangeEvent, error)
}

type AnalyticsRepository interface {
	GetMTTR(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsMTTR, error)
	GetIncidentTrends(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsIncidentTrend, error)
	GetAgentEffectiveness(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsAgentEffectiveness, error)
	GetCostByModel(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsCostByModel, error)
	GetCostByIncident(ctx context.Context, orgID uuid.UUID, limit int) ([]models.AnalyticsCostByIncident, error)
}

// --- Feedback Service ---

type FeedbackService struct{ repo FeedbackRepository }

func NewFeedbackService(repo FeedbackRepository) *FeedbackService {
	return &FeedbackService{repo: repo}
}

func (s *FeedbackService) Create(ctx context.Context, incidentID uuid.UUID, userID *uuid.UUID, req models.CreateFeedbackRequest) (*models.IncidentFeedback, error) {
	f := &models.IncidentFeedback{
		ID:            uuid.New(),
		IncidentID:    incidentID,
		UserID:        userID,
		TargetType:    req.TargetType,
		Rating:        req.Rating,
		Correction:    req.Correction,
		OriginalData:  req.OriginalData,
		CorrectedData: req.CorrectedData,
		CreatedAt:     time.Now(),
	}
	if f.OriginalData == nil {
		f.OriginalData = json.RawMessage("{}")
	}
	if f.CorrectedData == nil {
		f.CorrectedData = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *FeedbackService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentFeedback, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

// --- Knowledge Base Service ---

// Semantic search tuning: minimum cosine similarity and result cap.
const (
	kbSearchMinSimilarity = 0.3
	kbSearchLimit         = 10
)

type KnowledgeBaseService struct {
	repo       KnowledgeBaseRepository
	vectorRepo KnowledgeBaseVectorRepository // nil when repo has no vector support
	embedder   embeddings.Embedder           // nil when embeddings are disabled
}

func NewKnowledgeBaseService(repo KnowledgeBaseRepository) *KnowledgeBaseService {
	s := &KnowledgeBaseService{repo: repo}
	if vr, ok := repo.(KnowledgeBaseVectorRepository); ok {
		s.vectorRepo = vr
	}
	return s
}

// SetEmbedder enables semantic (embedding-based) search and write-path
// embedding computation. A nil embedder keeps everything on the ILIKE fallback.
func (s *KnowledgeBaseService) SetEmbedder(e embeddings.Embedder) {
	s.embedder = e
}

// embeddingText builds the text embedded for a knowledge base entry.
func kbEmbeddingText(kb *models.KnowledgeBaseEntry) string {
	parts := []string{}
	for _, p := range []string{kb.ErrorPattern, kb.RootCauseSummary, kb.ResolutionSummary} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	return strings.Join(parts, "\n")
}

// storeEmbedding computes and persists the entry's embedding, best-effort:
// failures are logged as warnings and never fail the write.
func (s *KnowledgeBaseService) storeEmbedding(ctx context.Context, kb *models.KnowledgeBaseEntry) {
	if s.embedder == nil || s.vectorRepo == nil {
		return
	}
	text := kbEmbeddingText(kb)
	if text == "" {
		return
	}
	vec, err := s.embedder.Embed(ctx, text)
	if err != nil {
		log.Printf("WARN: knowledge base %s: embedding computation failed: %v", kb.ID, err)
		return
	}
	if err := s.vectorRepo.UpdateEmbedding(ctx, kb.ID, vec); err != nil {
		log.Printf("WARN: knowledge base %s: embedding store failed: %v", kb.ID, err)
	}
}

func (s *KnowledgeBaseService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseEntry, error) {
	now := time.Now()
	kb := &models.KnowledgeBaseEntry{
		ID:                uuid.New(),
		OrgID:             orgID,
		IncidentID:        req.IncidentID,
		SoftwareID:        req.SoftwareID,
		Category:          req.Category,
		ErrorPattern:      req.ErrorPattern,
		RootCauseSummary:  req.RootCauseSummary,
		ResolutionSummary: req.ResolutionSummary,
		LessonsLearned:    req.LessonsLearned,
		ActionItems:       req.ActionItems,
		Tags:              req.Tags,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if kb.LessonsLearned == nil {
		kb.LessonsLearned = json.RawMessage("[]")
	}
	if kb.ActionItems == nil {
		kb.ActionItems = json.RawMessage("[]")
	}
	if kb.Tags == nil {
		kb.Tags = json.RawMessage("[]")
	}
	if err := s.repo.Create(ctx, kb); err != nil {
		return nil, err
	}
	s.storeEmbedding(ctx, kb)
	return kb, nil
}

// CreateFromHumanCorrection promotes a human's negative-feedback
// correction (see FeaturesHandler.CreateFeedback) into a knowledge-base
// entry. Deliberately bypasses CreateKnowledgeBaseRequest -- that DTO has
// no HumanValidated/Confidence fields at all (it's the generic API-facing
// create path, also used for agent-service's automatic post-incident
// write, which must NOT default to human_validated) -- so this is the one
// path that can actually set human_validated=true, confidence=1.0 the way
// those fields were always meant to be used.
func (s *KnowledgeBaseService) CreateFromHumanCorrection(ctx context.Context, orgID, incidentID uuid.UUID, softwareID *uuid.UUID, rootCauseSummary string) (*models.KnowledgeBaseEntry, error) {
	now := time.Now()
	pattern := rootCauseSummary
	if len(pattern) > 500 {
		pattern = pattern[:500]
	}
	kb := &models.KnowledgeBaseEntry{
		ID:               uuid.New(),
		OrgID:            orgID,
		IncidentID:       &incidentID,
		SoftwareID:       softwareID,
		Category:         "human_correction",
		ErrorPattern:     pattern,
		RootCauseSummary: rootCauseSummary,
		HumanValidated:   true,
		Confidence:       1.0,
		LessonsLearned:   json.RawMessage("[]"),
		ActionItems:      json.RawMessage("[]"),
		Tags:             json.RawMessage(`["human_validated"]`),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.Create(ctx, kb); err != nil {
		return nil, err
	}
	s.storeEmbedding(ctx, kb)
	return kb, nil
}

func (s *KnowledgeBaseService) GetByID(ctx context.Context, id uuid.UUID) (*models.KnowledgeBaseEntry, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *KnowledgeBaseService) List(ctx context.Context, orgID uuid.UUID, category string) ([]models.KnowledgeBaseEntry, error) {
	return s.repo.List(ctx, orgID, category)
}

// Search performs semantic (embedding-based) search when an embedder is
// configured and query text is present; otherwise — or when embedding the
// query fails — it falls back to the ILIKE substring match on error_pattern.
func (s *KnowledgeBaseService) Search(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, errorPattern string) ([]models.KnowledgeBaseEntry, error) {
	if s.embedder != nil && s.vectorRepo != nil && strings.TrimSpace(errorPattern) != "" {
		vec, err := s.embedder.Embed(ctx, errorPattern)
		if err != nil {
			log.Printf("WARN: knowledge base search: query embedding failed, falling back to ILIKE: %v", err)
		} else {
			items, err := s.vectorRepo.SearchByEmbedding(ctx, orgID, softwareID, vec, kbSearchMinSimilarity, kbSearchLimit)
			if err != nil {
				log.Printf("WARN: knowledge base search: vector search failed, falling back to ILIKE: %v", err)
			} else {
				return items, nil
			}
		}
	}
	return s.repo.Search(ctx, orgID, softwareID, errorPattern)
}

func (s *KnowledgeBaseService) Update(ctx context.Context, id uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseEntry, error) {
	kb, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	kb.Category = req.Category
	kb.ErrorPattern = req.ErrorPattern
	kb.RootCauseSummary = req.RootCauseSummary
	kb.ResolutionSummary = req.ResolutionSummary
	if req.LessonsLearned != nil {
		kb.LessonsLearned = req.LessonsLearned
	}
	if req.ActionItems != nil {
		kb.ActionItems = req.ActionItems
	}
	if req.Tags != nil {
		kb.Tags = req.Tags
	}
	kb.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, kb); err != nil {
		return nil, err
	}
	s.storeEmbedding(ctx, kb)
	return kb, nil
}

func (s *KnowledgeBaseService) IncrementReferences(ctx context.Context, id uuid.UUID) error {
	return s.repo.IncrementReferences(ctx, id)
}

// --- Similar Incident Service ---

type SimilarIncidentService struct {
	repo       SimilarIncidentRepository
	vectorRepo IncidentVectorRepository // nil when vector search is unavailable
}

func NewSimilarIncidentService(repo SimilarIncidentRepository) *SimilarIncidentService {
	return &SimilarIncidentService{repo: repo}
}

// SetVectorRepo enables embedding-based similar-incident matching.
func (s *SimilarIncidentService) SetVectorRepo(vr IncidentVectorRepository) {
	s.vectorRepo = vr
}

// FindSimilarByEmbedding returns embedding-based nearest incidents (same org,
// excluding the incident itself). Returns an empty slice when vector search
// is unavailable.
func (s *SimilarIncidentService) FindSimilarByEmbedding(ctx context.Context, incidentID uuid.UUID, limit int) ([]models.SimilarIncidentMatch, error) {
	if s.vectorRepo == nil {
		return []models.SimilarIncidentMatch{}, nil
	}
	return s.vectorRepo.FindSimilar(ctx, incidentID, limit)
}

func (s *SimilarIncidentService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateSimilarIncidentRequest) (*models.SimilarIncident, error) {
	si := &models.SimilarIncident{
		ID:                uuid.New(),
		IncidentID:        incidentID,
		SimilarIncidentID: req.SimilarIncidentID,
		SimilarityScore:   req.SimilarityScore,
		MatchedOn:         req.MatchedOn,
		CreatedAt:         time.Now(),
	}
	if si.MatchedOn == nil {
		si.MatchedOn = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, si); err != nil {
		return nil, err
	}
	return si, nil
}

func (s *SimilarIncidentService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.SimilarIncident, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

// --- Correlation Rule Service ---

type CorrelationRuleService struct{ repo CorrelationRuleRepository }

func NewCorrelationRuleService(repo CorrelationRuleRepository) *CorrelationRuleService {
	return &CorrelationRuleService{repo: repo}
}

func (s *CorrelationRuleService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateCorrelationRuleRequest) (*models.CorrelationRule, error) {
	now := time.Now()
	cr := &models.CorrelationRule{
		ID:                uuid.New(),
		OrgID:             orgID,
		Name:              req.Name,
		Description:       req.Description,
		RuleType:          req.RuleType,
		Config:            req.Config,
		TimeWindowSeconds: req.TimeWindowSeconds,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if cr.Config == nil {
		cr.Config = json.RawMessage("{}")
	}
	if cr.TimeWindowSeconds == 0 {
		cr.TimeWindowSeconds = 300
	}
	if err := s.repo.Create(ctx, cr); err != nil {
		return nil, err
	}
	return cr, nil
}

func (s *CorrelationRuleService) GetByID(ctx context.Context, id uuid.UUID) (*models.CorrelationRule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CorrelationRuleService) List(ctx context.Context, orgID uuid.UUID) ([]models.CorrelationRule, error) {
	return s.repo.List(ctx, orgID)
}

func (s *CorrelationRuleService) Update(ctx context.Context, id uuid.UUID, req models.CreateCorrelationRuleRequest) (*models.CorrelationRule, error) {
	cr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cr.Name = req.Name
	cr.Description = req.Description
	cr.RuleType = req.RuleType
	if req.Config != nil {
		cr.Config = req.Config
	}
	if req.TimeWindowSeconds > 0 {
		cr.TimeWindowSeconds = req.TimeWindowSeconds
	}
	cr.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cr); err != nil {
		return nil, err
	}
	return cr, nil
}

func (s *CorrelationRuleService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- Alert Group Service ---

type AlertGroupService struct{ repo AlertGroupRepository }

func NewAlertGroupService(repo AlertGroupRepository) *AlertGroupService {
	return &AlertGroupService{repo: repo}
}

func (s *AlertGroupService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateAlertGroupRequest) (*models.AlertGroup, error) {
	ag := &models.AlertGroup{
		ID:                uuid.New(),
		IncidentID:        incidentID,
		AlertSnapshotID:   req.AlertSnapshotID,
		CorrelationRuleID: req.CorrelationRuleID,
		CreatedAt:         time.Now(),
	}
	if err := s.repo.Create(ctx, ag); err != nil {
		return nil, err
	}
	return ag, nil
}

func (s *AlertGroupService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AlertGroup, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

// --- Notification Channel Service ---

type NotificationChannelService struct{ repo NotificationChannelRepository }

func NewNotificationChannelService(repo NotificationChannelRepository) *NotificationChannelService {
	return &NotificationChannelService{repo: repo}
}

func (s *NotificationChannelService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateNotificationChannelRequest) (*models.NotificationChannel, error) {
	now := time.Now()
	nc := &models.NotificationChannel{
		ID:          uuid.New(),
		OrgID:       orgID,
		Name:        req.Name,
		ChannelType: req.ChannelType,
		Config:      req.Config,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if nc.Config == nil {
		nc.Config = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, nc); err != nil {
		return nil, err
	}
	return nc, nil
}

func (s *NotificationChannelService) GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationChannel, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *NotificationChannelService) List(ctx context.Context, orgID uuid.UUID) ([]models.NotificationChannel, error) {
	return s.repo.List(ctx, orgID)
}

func (s *NotificationChannelService) Update(ctx context.Context, id uuid.UUID, req models.CreateNotificationChannelRequest) (*models.NotificationChannel, error) {
	nc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	nc.Name = req.Name
	nc.ChannelType = req.ChannelType
	if req.Config != nil {
		nc.Config = req.Config
	}
	nc.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, nc); err != nil {
		return nil, err
	}
	return nc, nil
}

func (s *NotificationChannelService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- Escalation Policy Service ---

type EscalationPolicyService struct{ repo EscalationPolicyRepository }

func NewEscalationPolicyService(repo EscalationPolicyRepository) *EscalationPolicyService {
	return &EscalationPolicyService{repo: repo}
}

func (s *EscalationPolicyService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateEscalationPolicyRequest) (*models.EscalationPolicy, error) {
	now := time.Now()
	ep := &models.EscalationPolicy{
		ID:                 uuid.New(),
		OrgID:              orgID,
		Name:               req.Name,
		Description:        req.Description,
		SoftwareID:         req.SoftwareID,
		SeverityFilter:     req.SeverityFilter,
		Steps:              req.Steps,
		RepeatAfterSeconds: req.RepeatAfterSeconds,
		Enabled:            true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if ep.SeverityFilter == nil {
		ep.SeverityFilter = json.RawMessage(`["critical","high","medium","low"]`)
	}
	if ep.Steps == nil {
		ep.Steps = json.RawMessage("[]")
	}
	if err := s.repo.Create(ctx, ep); err != nil {
		return nil, err
	}
	return ep, nil
}

func (s *EscalationPolicyService) GetByID(ctx context.Context, id uuid.UUID) (*models.EscalationPolicy, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EscalationPolicyService) List(ctx context.Context, orgID uuid.UUID) ([]models.EscalationPolicy, error) {
	return s.repo.List(ctx, orgID)
}

func (s *EscalationPolicyService) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.EscalationPolicy, error) {
	return s.repo.ListBySoftware(ctx, softwareID)
}

func (s *EscalationPolicyService) Update(ctx context.Context, id uuid.UUID, req models.CreateEscalationPolicyRequest) (*models.EscalationPolicy, error) {
	ep, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ep.Name = req.Name
	ep.Description = req.Description
	ep.SoftwareID = req.SoftwareID
	if req.SeverityFilter != nil {
		ep.SeverityFilter = req.SeverityFilter
	}
	if req.Steps != nil {
		ep.Steps = req.Steps
	}
	ep.RepeatAfterSeconds = req.RepeatAfterSeconds
	ep.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, ep); err != nil {
		return nil, err
	}
	return ep, nil
}

func (s *EscalationPolicyService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- Notification Log Service ---

type NotificationLogService struct{ repo NotificationLogRepository }

func NewNotificationLogService(repo NotificationLogRepository) *NotificationLogService {
	return &NotificationLogService{repo: repo}
}

func (s *NotificationLogService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateNotificationLogRequest) (*models.NotificationLogEntry, error) {
	nl := &models.NotificationLogEntry{
		ID:         uuid.New(),
		OrgID:      orgID,
		IncidentID: req.IncidentID,
		ChannelID:  req.ChannelID,
		PolicyID:   req.PolicyID,
		EventType:  req.EventType,
		Recipient:  req.Recipient,
		Payload:    req.Payload,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	if nl.Payload == nil {
		nl.Payload = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, nl); err != nil {
		return nil, err
	}
	return nl, nil
}

func (s *NotificationLogService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.NotificationLogEntry, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

func (s *NotificationLogService) ListByOrg(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.NotificationLogEntry, int, error) {
	return s.repo.ListByOrg(ctx, orgID, page, perPage)
}

func (s *NotificationLogService) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errMsg string) error {
	return s.repo.UpdateStatus(ctx, id, status, errMsg)
}

// --- Runbook Service ---

type RunbookService struct{ repo RunbookRepository }

func NewRunbookService(repo RunbookRepository) *RunbookService {
	return &RunbookService{repo: repo}
}

func (s *RunbookService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateRunbookRequest) (*models.Runbook, error) {
	now := time.Now()
	rb := &models.Runbook{
		ID:                uuid.New(),
		OrgID:             orgID,
		SoftwareID:        req.SoftwareID,
		Name:              req.Name,
		Slug:              req.Slug,
		Description:       req.Description,
		TriggerConditions: req.TriggerConditions,
		AutoTrigger:       req.AutoTrigger,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if rb.TriggerConditions == nil {
		rb.TriggerConditions = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, rb); err != nil {
		return nil, err
	}
	return rb, nil
}

func (s *RunbookService) GetByID(ctx context.Context, id uuid.UUID) (*models.Runbook, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RunbookService) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Runbook, error) {
	return s.repo.GetBySlug(ctx, orgID, slug)
}

func (s *RunbookService) List(ctx context.Context, orgID uuid.UUID) ([]models.Runbook, error) {
	return s.repo.List(ctx, orgID)
}

func (s *RunbookService) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.Runbook, error) {
	return s.repo.ListBySoftware(ctx, softwareID)
}

func (s *RunbookService) Update(ctx context.Context, id uuid.UUID, req models.CreateRunbookRequest) (*models.Runbook, error) {
	rb, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rb.SoftwareID = req.SoftwareID
	rb.Name = req.Name
	rb.Slug = req.Slug
	rb.Description = req.Description
	if req.TriggerConditions != nil {
		rb.TriggerConditions = req.TriggerConditions
	}
	rb.AutoTrigger = req.AutoTrigger
	rb.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, rb); err != nil {
		return nil, err
	}
	return rb, nil
}

func (s *RunbookService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- Runbook Step Service ---

type RunbookStepService struct{ repo RunbookStepRepository }

func NewRunbookStepService(repo RunbookStepRepository) *RunbookStepService {
	return &RunbookStepService{repo: repo}
}

func (s *RunbookStepService) Create(ctx context.Context, runbookID uuid.UUID, req models.CreateRunbookStepRequest) (*models.RunbookStep, error) {
	step := &models.RunbookStep{
		ID:             uuid.New(),
		RunbookID:      runbookID,
		StepOrder:      req.StepOrder,
		Name:           req.Name,
		Description:    req.Description,
		StepType:       req.StepType,
		Config:         req.Config,
		SkillID:        req.SkillID,
		TimeoutSeconds: req.TimeoutSeconds,
		OnFailure:      req.OnFailure,
		MaxRetries:     req.MaxRetries,
		CreatedAt:      time.Now(),
	}
	if step.Config == nil {
		step.Config = json.RawMessage("{}")
	}
	if step.OnFailure == "" {
		step.OnFailure = "stop"
	}
	if step.TimeoutSeconds == 0 {
		step.TimeoutSeconds = 300
	}
	if err := s.repo.Create(ctx, step); err != nil {
		return nil, err
	}
	return step, nil
}

func (s *RunbookStepService) ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookStep, error) {
	return s.repo.ListByRunbook(ctx, runbookID)
}

func (s *RunbookStepService) Update(ctx context.Context, id uuid.UUID, req models.CreateRunbookStepRequest) (*models.RunbookStep, error) {
	// We need to construct the step from the request since we don't have a GetByID
	step := &models.RunbookStep{
		ID:             id,
		StepOrder:      req.StepOrder,
		Name:           req.Name,
		Description:    req.Description,
		StepType:       req.StepType,
		Config:         req.Config,
		SkillID:        req.SkillID,
		TimeoutSeconds: req.TimeoutSeconds,
		OnFailure:      req.OnFailure,
		MaxRetries:     req.MaxRetries,
	}
	if step.Config == nil {
		step.Config = json.RawMessage("{}")
	}
	if err := s.repo.Update(ctx, step); err != nil {
		return nil, err
	}
	return step, nil
}

func (s *RunbookStepService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Reorder sets each step's step_order to its index in orderedStepIDs.
// Found missing live: the frontend already had a drag-to-reorder UI
// (reorderRunbookSteps in api.ts) wired to a route that never existed on
// the backend -- reordering was a no-op stacked on both sides.
func (s *RunbookStepService) Reorder(ctx context.Context, orderedStepIDs []uuid.UUID) error {
	for i, stepID := range orderedStepIDs {
		if err := s.repo.UpdateOrder(ctx, stepID, i); err != nil {
			return err
		}
	}
	return nil
}

// --- Runbook Execution Service ---

type RunbookExecutionService struct{ repo RunbookExecutionRepository }

func NewRunbookExecutionService(repo RunbookExecutionRepository) *RunbookExecutionService {
	return &RunbookExecutionService{repo: repo}
}

func (s *RunbookExecutionService) Create(ctx context.Context, req models.CreateRunbookExecutionRequest) (*models.RunbookExecution, error) {
	now := time.Now()
	re := &models.RunbookExecution{
		ID:          uuid.New(),
		RunbookID:   req.RunbookID,
		IncidentID:  req.IncidentID,
		TriggeredBy: req.TriggeredBy,
		Status:      "pending",
		StepResults: json.RawMessage("[]"),
		StartedAt:   &now,
		CreatedAt:   now,
	}
	if err := s.repo.Create(ctx, re); err != nil {
		return nil, err
	}
	return re, nil
}

func (s *RunbookExecutionService) GetByID(ctx context.Context, id uuid.UUID) (*models.RunbookExecution, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RunbookExecutionService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.RunbookExecution, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

func (s *RunbookExecutionService) ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookExecution, error) {
	return s.repo.ListByRunbook(ctx, runbookID)
}

func (s *RunbookExecutionService) Update(ctx context.Context, id uuid.UUID, req models.UpdateRunbookExecutionRequest) (*models.RunbookExecution, error) {
	re, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		re.Status = *req.Status
		if *req.Status == "completed" || *req.Status == "failed" || *req.Status == "canceled" {
			now := time.Now()
			re.CompletedAt = &now
		}
	}
	if req.CurrentStep != nil {
		re.CurrentStep = *req.CurrentStep
	}
	if req.StepResults != nil {
		re.StepResults = req.StepResults
	}
	if err := s.repo.Update(ctx, re); err != nil {
		return nil, err
	}
	return re, nil
}

// --- Change Event Service ---

type ChangeEventService struct{ repo ChangeEventRepository }

func NewChangeEventService(repo ChangeEventRepository) *ChangeEventService {
	return &ChangeEventService{repo: repo}
}

func (s *ChangeEventService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateChangeEventRequest) (*models.ChangeEvent, error) {
	now := time.Now()
	ce := &models.ChangeEvent{
		ID:          uuid.New(),
		OrgID:       orgID,
		SoftwareID:  req.SoftwareID,
		ChangeType:  req.ChangeType,
		Title:       req.Title,
		Description: req.Description,
		Source:      req.Source,
		SourceURL:   req.SourceURL,
		CommitSHA:   req.CommitSHA,
		Author:      req.Author,
		Environment: req.Environment,
		Metadata:    req.Metadata,
		OccurredAt:  now,
		CreatedAt:   now,
	}
	if req.OccurredAt != nil {
		ce.OccurredAt = *req.OccurredAt
	}
	if ce.Metadata == nil {
		ce.Metadata = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, ce); err != nil {
		return nil, err
	}
	return ce, nil
}

func (s *ChangeEventService) ListBySoftware(ctx context.Context, softwareID uuid.UUID, since time.Time) ([]models.ChangeEvent, error) {
	return s.repo.ListBySoftware(ctx, softwareID, since)
}

func (s *ChangeEventService) ListRecent(ctx context.Context, softwareID uuid.UUID, minutes int) ([]models.ChangeEvent, error) {
	return s.repo.ListRecent(ctx, softwareID, minutes)
}

// --- Analytics Service ---

type AnalyticsService struct{ repo AnalyticsRepository }

func NewAnalyticsService(repo AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) GetMTTR(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsMTTR, error) {
	return s.repo.GetMTTR(ctx, orgID, days)
}

func (s *AnalyticsService) GetIncidentTrends(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsIncidentTrend, error) {
	return s.repo.GetIncidentTrends(ctx, orgID, days)
}

func (s *AnalyticsService) GetAgentEffectiveness(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsAgentEffectiveness, error) {
	return s.repo.GetAgentEffectiveness(ctx, orgID)
}

func (s *AnalyticsService) GetCostByModel(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsCostByModel, error) {
	return s.repo.GetCostByModel(ctx, orgID)
}

func (s *AnalyticsService) GetCostByIncident(ctx context.Context, orgID uuid.UUID, limit int) ([]models.AnalyticsCostByIncident, error) {
	return s.repo.GetCostByIncident(ctx, orgID, limit)
}
