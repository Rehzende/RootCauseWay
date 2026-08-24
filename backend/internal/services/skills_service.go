package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Repository interfaces for Skills services

type SkillRepository interface {
	Create(ctx context.Context, s *models.Skill) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Skill, error)
	List(ctx context.Context, orgID uuid.UUID, category string, page, perPage int) ([]models.Skill, int, error)
	Update(ctx context.Context, s *models.Skill) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AgentSkillRepository interface {
	Link(ctx context.Context, link *models.AgentSkillLink) error
	Unlink(ctx context.Context, agentID, skillID uuid.UUID) error
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.AgentSkillLink, error)
	ListBySkill(ctx context.Context, skillID uuid.UUID) ([]models.AgentSkillLink, error)
}

type CredentialProviderRepository interface {
	Create(ctx context.Context, p *models.CredentialProvider) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialProvider, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.CredentialProvider, int, error)
	Update(ctx context.Context, p *models.CredentialProvider) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ResourceCredentialRepository interface {
	Create(ctx context.Context, rc *models.ResourceCredential) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ResourceCredential, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ResourceCredential, error)
	ListByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ResourceCredential, error)
	Update(ctx context.Context, rc *models.ResourceCredential) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AccessPolicyRepository interface {
	Create(ctx context.Context, p *models.AccessPolicy) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AccessPolicy, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.AccessPolicy, int, error)
	ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]models.AccessPolicy, error)
	Update(ctx context.Context, p *models.AccessPolicy) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CredentialLeaseRepository interface {
	Create(ctx context.Context, l *models.CredentialLease) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialLease, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.CredentialLease, error)
	ListActive(ctx context.Context, orgID uuid.UUID) ([]models.CredentialLease, error)
	Update(ctx context.Context, l *models.CredentialLease) error
	ExpireLeases(ctx context.Context) (int64, error)
}

// --- SkillService ---

type SkillService struct {
	repo SkillRepository
}

func NewSkillService(repo SkillRepository) *SkillService {
	return &SkillService{repo: repo}
}

func (s *SkillService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSkillRequest) (*models.Skill, error) {
	now := time.Now()
	skill := &models.Skill{
		ID:                    uuid.New(),
		OrgID:                 orgID,
		Name:                  req.Name,
		Slug:                  req.Slug,
		Description:           req.Description,
		Category:              req.Category,
		PromptTemplate:        req.PromptTemplate,
		InputSchema:           req.InputSchema,
		OutputSchema:          req.OutputSchema,
		RequiredTools:         req.RequiredTools,
		RequiredResourceTypes: req.RequiredResourceTypes,
		RequiredPermissions:   req.RequiredPermissions,
		Enabled:               true,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if skill.InputSchema == nil {
		skill.InputSchema = json.RawMessage("{}")
	}
	if skill.OutputSchema == nil {
		skill.OutputSchema = json.RawMessage("{}")
	}
	if skill.RequiredTools == nil {
		skill.RequiredTools = json.RawMessage("[]")
	}
	if skill.RequiredResourceTypes == nil {
		skill.RequiredResourceTypes = json.RawMessage("[]")
	}
	if skill.RequiredPermissions == nil {
		skill.RequiredPermissions = json.RawMessage("[]")
	}
	if err := s.repo.Create(ctx, skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *SkillService) GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SkillService) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Skill, error) {
	return s.repo.GetBySlug(ctx, orgID, slug)
}

func (s *SkillService) List(ctx context.Context, orgID uuid.UUID, category string, page, perPage int) ([]models.Skill, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, category, page, perPage)
}

func (s *SkillService) Update(ctx context.Context, id uuid.UUID, req models.CreateSkillRequest) (*models.Skill, error) {
	skill, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	skill.Name = req.Name
	skill.Slug = req.Slug
	skill.Description = req.Description
	skill.Category = req.Category
	skill.PromptTemplate = req.PromptTemplate
	if req.InputSchema != nil {
		skill.InputSchema = req.InputSchema
	}
	if req.OutputSchema != nil {
		skill.OutputSchema = req.OutputSchema
	}
	if req.RequiredTools != nil {
		skill.RequiredTools = req.RequiredTools
	}
	if req.RequiredResourceTypes != nil {
		skill.RequiredResourceTypes = req.RequiredResourceTypes
	}
	if req.RequiredPermissions != nil {
		skill.RequiredPermissions = req.RequiredPermissions
	}
	if req.Enabled != nil {
		skill.Enabled = *req.Enabled
	}
	skill.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, skill); err != nil {
		return nil, err
	}
	return skill, nil
}

func (s *SkillService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- AgentSkillService ---

type AgentSkillService struct {
	repo AgentSkillRepository
}

func NewAgentSkillService(repo AgentSkillRepository) *AgentSkillService {
	return &AgentSkillService{repo: repo}
}

func (s *AgentSkillService) Link(ctx context.Context, agentID uuid.UUID, req models.CreateAgentSkillLinkRequest) (*models.AgentSkillLink, error) {
	link := &models.AgentSkillLink{
		ID:              uuid.New(),
		AgentID:         agentID,
		SkillID:         req.SkillID,
		Priority:        req.Priority,
		ConfigOverrides: req.ConfigOverrides,
		CreatedAt:       time.Now(),
	}
	if link.ConfigOverrides == nil {
		link.ConfigOverrides = json.RawMessage("{}")
	}
	if err := s.repo.Link(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

func (s *AgentSkillService) Unlink(ctx context.Context, agentID, skillID uuid.UUID) error {
	return s.repo.Unlink(ctx, agentID, skillID)
}

func (s *AgentSkillService) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.AgentSkillLink, error) {
	return s.repo.ListByAgent(ctx, agentID)
}

func (s *AgentSkillService) ListBySkill(ctx context.Context, skillID uuid.UUID) ([]models.AgentSkillLink, error) {
	return s.repo.ListBySkill(ctx, skillID)
}

// --- CredentialProviderService ---

type CredentialProviderService struct {
	repo CredentialProviderRepository
}

func NewCredentialProviderService(repo CredentialProviderRepository) *CredentialProviderService {
	return &CredentialProviderService{repo: repo}
}

func (s *CredentialProviderService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateCredentialProviderRequest) (*models.CredentialProvider, error) {
	now := time.Now()
	provider := &models.CredentialProvider{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         req.Name,
		ProviderType: req.ProviderType,
		Config:       req.Config,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if provider.Config == nil {
		provider.Config = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, provider); err != nil {
		return nil, err
	}
	return provider, nil
}

func (s *CredentialProviderService) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialProvider, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CredentialProviderService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.CredentialProvider, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, page, perPage)
}

func (s *CredentialProviderService) Update(ctx context.Context, id uuid.UUID, req models.CreateCredentialProviderRequest) (*models.CredentialProvider, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	provider.Name = req.Name
	provider.ProviderType = req.ProviderType
	if req.Config != nil {
		provider.Config = req.Config
	}
	provider.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, provider); err != nil {
		return nil, err
	}
	return provider, nil
}

func (s *CredentialProviderService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- ResourceCredentialService ---

type ResourceCredentialService struct {
	repo ResourceCredentialRepository
}

func NewResourceCredentialService(repo ResourceCredentialRepository) *ResourceCredentialService {
	return &ResourceCredentialService{repo: repo}
}

func (s *ResourceCredentialService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateResourceCredentialRequest) (*models.ResourceCredential, error) {
	now := time.Now()
	rc := &models.ResourceCredential{
		ID:             uuid.New(),
		OrgID:          orgID,
		SoftwareID:     req.SoftwareID,
		ResourceName:   req.ResourceName,
		ResourceType:   req.ResourceType,
		ProviderID:     req.ProviderID,
		CredentialPath: req.CredentialPath,
		DefaultTTL:     req.DefaultTTL,
		MaxTTL:         req.MaxTTL,
		DefaultScope:   req.DefaultScope,
		Metadata:       req.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if rc.DefaultScope == nil {
		rc.DefaultScope = json.RawMessage("{}")
	}
	if rc.Metadata == nil {
		rc.Metadata = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (s *ResourceCredentialService) GetByID(ctx context.Context, id uuid.UUID) (*models.ResourceCredential, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ResourceCredentialService) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ResourceCredential, error) {
	return s.repo.ListBySoftware(ctx, softwareID)
}

func (s *ResourceCredentialService) Update(ctx context.Context, id uuid.UUID, req models.CreateResourceCredentialRequest) (*models.ResourceCredential, error) {
	rc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rc.ResourceName = req.ResourceName
	rc.ResourceType = req.ResourceType
	rc.ProviderID = req.ProviderID
	rc.CredentialPath = req.CredentialPath
	rc.DefaultTTL = req.DefaultTTL
	rc.MaxTTL = req.MaxTTL
	if req.DefaultScope != nil {
		rc.DefaultScope = req.DefaultScope
	}
	if req.Metadata != nil {
		rc.Metadata = req.Metadata
	}
	rc.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (s *ResourceCredentialService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- AccessPolicyService ---

type AccessPolicyService struct {
	repo AccessPolicyRepository
}

func NewAccessPolicyService(repo AccessPolicyRepository) *AccessPolicyService {
	return &AccessPolicyService{repo: repo}
}

func (s *AccessPolicyService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateAccessPolicyRequest) (*models.AccessPolicy, error) {
	now := time.Now()
	policy := &models.AccessPolicy{
		ID:                uuid.New(),
		OrgID:             orgID,
		Name:              req.Name,
		Description:       req.Description,
		TargetType:        req.TargetType,
		TargetID:          req.TargetID,
		ResourceType:      req.ResourceType,
		AllowedActions:    req.AllowedActions,
		ScopeRestrictions: req.ScopeRestrictions,
		MaxTTL:            req.MaxTTL,
		RequireApproval:   req.RequireApproval,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if policy.AllowedActions == nil {
		policy.AllowedActions = json.RawMessage("[]")
	}
	if policy.ScopeRestrictions == nil {
		policy.ScopeRestrictions = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *AccessPolicyService) GetByID(ctx context.Context, id uuid.UUID) (*models.AccessPolicy, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AccessPolicyService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.AccessPolicy, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, page, perPage)
}

func (s *AccessPolicyService) ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]models.AccessPolicy, error) {
	return s.repo.ListByTarget(ctx, targetType, targetID)
}

func (s *AccessPolicyService) Update(ctx context.Context, id uuid.UUID, req models.CreateAccessPolicyRequest) (*models.AccessPolicy, error) {
	policy, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	policy.Name = req.Name
	policy.Description = req.Description
	policy.TargetType = req.TargetType
	policy.TargetID = req.TargetID
	policy.ResourceType = req.ResourceType
	if req.AllowedActions != nil {
		policy.AllowedActions = req.AllowedActions
	}
	if req.ScopeRestrictions != nil {
		policy.ScopeRestrictions = req.ScopeRestrictions
	}
	policy.MaxTTL = req.MaxTTL
	policy.RequireApproval = req.RequireApproval
	policy.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func (s *AccessPolicyService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Evaluate finds matching policies for given agent, skill, and resource type.
func (s *AccessPolicyService) Evaluate(ctx context.Context, orgID uuid.UUID, agentID, skillID uuid.UUID, resourceType string) ([]models.AccessPolicy, error) {
	// Check policies targeting the agent directly
	agentPolicies, err := s.repo.ListByTarget(ctx, "agent", agentID)
	if err != nil {
		return nil, err
	}

	// Check policies targeting the skill
	skillPolicies, err := s.repo.ListByTarget(ctx, "skill", skillID)
	if err != nil {
		return nil, err
	}

	var matched []models.AccessPolicy
	for _, p := range agentPolicies {
		if p.ResourceType == resourceType || p.ResourceType == "*" {
			matched = append(matched, p)
		}
	}
	for _, p := range skillPolicies {
		if p.ResourceType == resourceType || p.ResourceType == "*" {
			matched = append(matched, p)
		}
	}
	if matched == nil {
		matched = []models.AccessPolicy{}
	}
	return matched, nil
}

// --- CredentialLeaseService ---

type CredentialLeaseService struct {
	leaseRepo    CredentialLeaseRepository
	policyRepo   AccessPolicyRepository
	rcRepo       ResourceCredentialRepository
	providerRepo CredentialProviderRepository
}

func NewCredentialLeaseService(leaseRepo CredentialLeaseRepository, policyRepo AccessPolicyRepository, rcRepo ResourceCredentialRepository, providerRepo CredentialProviderRepository) *CredentialLeaseService {
	return &CredentialLeaseService{leaseRepo: leaseRepo, policyRepo: policyRepo, rcRepo: rcRepo, providerRepo: providerRepo}
}

// ErrCredentialProviderNotImplemented is returned when a lease is requested
// against a provider_type this Go implementation doesn't actually know how
// to call yet. Audit found the whole credential vault -- every provider
// type, not just this one -- never generated real secret material at all:
// RequestLease built and persisted a lease row (audit metadata: who, when,
// TTL) but never populated CredentialData, and the Python provider classes
// (StaticProvider/VaultProvider/AWSSTSProvider/AzureMIProvider, all fully
// implemented) were never called from anywhere. "static" is wired for real
// below -- it's config-only, no external service to call, so it's safe to
// implement and verify without a live Vault/AWS/Azure/GCP account, which
// this deployment doesn't have. The other four now fail loudly with this
// error instead of silently returning an empty credential -- worse than
// before in the sense that a genuinely-configured Vault/AWS/Azure/GCP
// provider will error rather than silently no-op, but that's the honest
// state until each is actually implemented and can be verified against
// real infrastructure.
var ErrCredentialProviderNotImplemented = fmt.Errorf("credential provider type not implemented")

// credentialEnvelope builds the fields common to every provider's
// resolved credential data (lease bookkeeping, not secret material
// itself) -- factored out so azure_key_vault doesn't duplicate what
// "static" already established below.
func credentialEnvelope(providerType, credentialPath string, scope json.RawMessage, ttlSeconds int) map[string]interface{} {
	now := time.Now()
	data := map[string]interface{}{
		"credential_id":   uuid.New().String(),
		"provider":        providerType,
		"credential_path": credentialPath,
		"issued_at":       now.Unix(),
		"expires_at":      now.Add(time.Duration(ttlSeconds) * time.Second).Unix(),
	}
	var scopeMap map[string]interface{}
	if len(scope) > 0 {
		_ = json.Unmarshal(scope, &scopeMap)
	}
	data["scope"] = scopeMap
	return data
}

func ResolveCredentialData(ctx context.Context, provider *models.CredentialProvider, credentialPath string, scope json.RawMessage, ttlSeconds int) (map[string]interface{}, error) {
	switch provider.ProviderType {
	case "static":
		var cfg struct {
			Credentials map[string]interface{} `json:"credentials"`
		}
		if err := json.Unmarshal(provider.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid static provider config: %w", err)
		}
		data := credentialEnvelope("static", credentialPath, scope, ttlSeconds)
		for k, v := range cfg.Credentials {
			data[k] = v
		}
		return data, nil
	case "azure_key_vault":
		secretData, err := resolveAzureKeyVaultCredential(ctx, provider.Config, credentialPath)
		if err != nil {
			return nil, err
		}
		data := credentialEnvelope("azure_key_vault", credentialPath, scope, ttlSeconds)
		for k, v := range secretData {
			data[k] = v
		}
		return data, nil
	case "azure_aks_jit":
		aksData, err := resolveAzureAKSJITCredential(ctx, provider.Config, credentialPath)
		if err != nil {
			return nil, err
		}
		data := credentialEnvelope("azure_aks_jit", credentialPath, scope, ttlSeconds)
		for k, v := range aksData {
			data[k] = v
		}
		// The envelope's expires_at reflects the REQUESTED lease TTL --
		// this is real JIT (see project backlog design notes), so the
		// lease must never outlive the actual Azure AD token it wraps.
		// Cap it down (never extend it) to whichever is sooner.
		if tokenExpiresAt, ok := data["token_expires_at"].(int64); ok {
			if envelopeExpiry, ok := data["expires_at"].(int64); ok && tokenExpiresAt < envelopeExpiry {
				data["expires_at"] = tokenExpiresAt
			}
		}
		return data, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrCredentialProviderNotImplemented, provider.ProviderType)
	}
}

func (s *CredentialLeaseService) RequestLease(ctx context.Context, orgID uuid.UUID, req models.RequestLeaseRequest) (*models.CredentialLease, error) {
	// Get the resource credential to validate and get defaults
	rc, err := s.rcRepo.GetByID(ctx, req.ResourceCredentialID)
	if err != nil {
		return nil, fmt.Errorf("resource credential not found: %w", err)
	}

	// Determine TTL
	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = rc.DefaultTTL
	}
	if ttl <= 0 {
		ttl = 3600 // default 1 hour
	}
	if rc.MaxTTL > 0 && ttl > rc.MaxTTL {
		ttl = rc.MaxTTL
	}

	// Find matching policies
	agentID := req.AgentID
	var skillID uuid.UUID
	if req.SkillID != nil {
		skillID = *req.SkillID
	}

	// Check agent policies
	agentPolicies, _ := s.policyRepo.ListByTarget(ctx, "agent", agentID)
	var matchedPolicy *models.AccessPolicy
	for _, p := range agentPolicies {
		if (p.ResourceType == rc.ResourceType || p.ResourceType == "*") && p.Enabled {
			matchedPolicy = &p
			break
		}
	}

	// Check skill policies if no agent policy matched
	if matchedPolicy == nil && req.SkillID != nil {
		skillPolicies, _ := s.policyRepo.ListByTarget(ctx, "skill", skillID)
		for _, p := range skillPolicies {
			if (p.ResourceType == rc.ResourceType || p.ResourceType == "*") && p.Enabled {
				matchedPolicy = &p
				break
			}
		}
	}

	// Enforce policy max TTL
	if matchedPolicy != nil && matchedPolicy.MaxTTL > 0 && ttl > matchedPolicy.MaxTTL {
		ttl = matchedPolicy.MaxTTL
	}

	// Check if approval is required
	if matchedPolicy != nil && matchedPolicy.RequireApproval {
		return nil, fmt.Errorf("policy %s requires approval", matchedPolicy.Name)
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	scope := req.Scope
	if scope == nil {
		scope = rc.DefaultScope
	}

	var policyID *uuid.UUID
	if matchedPolicy != nil {
		policyID = &matchedPolicy.ID
	}

	lease := &models.CredentialLease{
		ID:                   uuid.New(),
		OrgID:                orgID,
		IncidentID:           req.IncidentID,
		AgentID:              req.AgentID,
		SkillID:              req.SkillID,
		ResourceCredentialID: req.ResourceCredentialID,
		PolicyID:             policyID,
		Status:               "active",
		Scope:                scope,
		IssuedAt:             &now,
		ExpiresAt:            &expiresAt,
		RevokedBy:            "",
		RequestReason:        req.Reason,
		ActionsPerformed:     json.RawMessage("[]"),
		CreatedAt:            now,
	}
	if lease.Scope == nil {
		lease.Scope = json.RawMessage("{}")
	}

	provider, err := s.providerRepo.GetByID(ctx, rc.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("credential provider not found: %w", err)
	}
	credentialData, err := ResolveCredentialData(ctx, provider, rc.CredentialPath, lease.Scope, ttl)
	if err != nil {
		// Don't persist a lease row implying an active credential exists
		// when generation actually failed/isn't implemented.
		return nil, err
	}

	if err := s.leaseRepo.Create(ctx, lease); err != nil {
		return nil, err
	}
	lease.CredentialData = credentialData
	return lease, nil
}

func (s *CredentialLeaseService) RevokeLease(ctx context.Context, id uuid.UUID, revokedBy string) (*models.CredentialLease, error) {
	lease, err := s.leaseRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	lease.Status = "revoked"
	lease.RevokedAt = &now
	lease.RevokedBy = revokedBy
	if err := s.leaseRepo.Update(ctx, lease); err != nil {
		return nil, err
	}
	return lease, nil
}

func (s *CredentialLeaseService) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialLease, error) {
	return s.leaseRepo.GetByID(ctx, id)
}

func (s *CredentialLeaseService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.CredentialLease, error) {
	return s.leaseRepo.ListByIncident(ctx, incidentID)
}

func (s *CredentialLeaseService) ListActive(ctx context.Context, orgID uuid.UUID) ([]models.CredentialLease, error) {
	return s.leaseRepo.ListActive(ctx, orgID)
}

func (s *CredentialLeaseService) ExpireStale(ctx context.Context) (int64, error) {
	return s.leaseRepo.ExpireLeases(ctx)
}
