package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A platform audit found the entire credential vault never generated real
// secret material for ANY provider type: RequestLease built and persisted
// a lease row (audit metadata only) but never populated CredentialData,
// and the Python provider implementations (StaticProvider/VaultProvider/
// AWSSTSProvider/AzureMIProvider) were never called from anywhere. These
// tests pin the fix for "static" (config-only, no external service, safe
// to implement/verify without live infra) and the loud-failure behavior
// for the other provider types, which aren't implemented yet.

type fakeLeaseRepo struct {
	created *models.CredentialLease
}

func (f *fakeLeaseRepo) Create(ctx context.Context, l *models.CredentialLease) error {
	f.created = l
	return nil
}
func (f *fakeLeaseRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialLease, error) {
	return nil, nil
}
func (f *fakeLeaseRepo) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.CredentialLease, error) {
	return nil, nil
}
func (f *fakeLeaseRepo) ListActive(ctx context.Context, orgID uuid.UUID) ([]models.CredentialLease, error) {
	return nil, nil
}
func (f *fakeLeaseRepo) Update(ctx context.Context, l *models.CredentialLease) error { return nil }
func (f *fakeLeaseRepo) ExpireLeases(ctx context.Context) (int64, error)             { return 0, nil }

type fakePolicyRepo struct{}

func (f *fakePolicyRepo) Create(ctx context.Context, p *models.AccessPolicy) error { return nil }
func (f *fakePolicyRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.AccessPolicy, error) {
	return nil, nil
}
func (f *fakePolicyRepo) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.AccessPolicy, int, error) {
	return nil, 0, nil
}
func (f *fakePolicyRepo) ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]models.AccessPolicy, error) {
	return nil, nil // no policies -- RequestLease proceeds without one, same as an unrestricted resource
}
func (f *fakePolicyRepo) Update(ctx context.Context, p *models.AccessPolicy) error { return nil }
func (f *fakePolicyRepo) Delete(ctx context.Context, id uuid.UUID) error           { return nil }

type fakeResourceCredentialRepo struct {
	rc *models.ResourceCredential
}

func (f *fakeResourceCredentialRepo) Create(ctx context.Context, rc *models.ResourceCredential) error {
	return nil
}
func (f *fakeResourceCredentialRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.ResourceCredential, error) {
	return f.rc, nil
}
func (f *fakeResourceCredentialRepo) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ResourceCredential, error) {
	return nil, nil
}
func (f *fakeResourceCredentialRepo) ListByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ResourceCredential, error) {
	return nil, nil
}
func (f *fakeResourceCredentialRepo) Update(ctx context.Context, rc *models.ResourceCredential) error {
	return nil
}
func (f *fakeResourceCredentialRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }

type fakeProviderRepo struct {
	provider *models.CredentialProvider
}

func (f *fakeProviderRepo) Create(ctx context.Context, p *models.CredentialProvider) error {
	return nil
}
func (f *fakeProviderRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialProvider, error) {
	return f.provider, nil
}
func (f *fakeProviderRepo) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.CredentialProvider, int, error) {
	return nil, 0, nil
}
func (f *fakeProviderRepo) Update(ctx context.Context, p *models.CredentialProvider) error {
	return nil
}
func (f *fakeProviderRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func TestRequestLease_StaticProvider_ResolvesRealCredentialData(t *testing.T) {
	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "database", CredentialPath: "db/prod", DefaultTTL: 300}
	provider := &models.CredentialProvider{
		ID: providerID, ProviderType: "static",
		Config: json.RawMessage(`{"credentials":{"username":"svc_user","password":"s3cr3t"}}`),
	}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	lease, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID,
	})

	require.NoError(t, err)
	require.NotNil(t, lease.CredentialData)
	assert.Equal(t, "svc_user", lease.CredentialData["username"])
	assert.Equal(t, "s3cr3t", lease.CredentialData["password"])
	assert.Equal(t, "static", lease.CredentialData["provider"])
	assert.Equal(t, "db/prod", lease.CredentialData["credential_path"])
	// The lease audit row must still get persisted -- CredentialData is
	// only ever attached to the in-memory response, never written to it.
	assert.NotNil(t, leaseRepo.created)
}

func TestRequestLease_UnimplementedProvider_FailsLoudAndDoesNotPersistLease(t *testing.T) {
	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "kubernetes_cluster", DefaultTTL: 300}
	provider := &models.CredentialProvider{ID: providerID, ProviderType: "hashicorp_vault", Config: json.RawMessage(`{}`)}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	_, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCredentialProviderNotImplemented))
	assert.Nil(t, leaseRepo.created, "must not persist a lease row for a credential that was never actually generated")
}
