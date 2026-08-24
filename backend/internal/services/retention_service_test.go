package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock ---

type MockRetentionRepo struct{ mock.Mock }

func (m *MockRetentionRepo) CreatePolicy(ctx context.Context, p *models.RetentionPolicy) error {
	return m.Called(ctx, p).Error(0)
}
func (m *MockRetentionRepo) GetPolicy(ctx context.Context, id uuid.UUID) (*models.RetentionPolicy, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionRepo) ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionRepo) ListEnabledPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionRepo) UpdatePolicy(ctx context.Context, p *models.RetentionPolicy) error {
	return m.Called(ctx, p).Error(0)
}
func (m *MockRetentionRepo) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockRetentionRepo) ListOrgIDs(ctx context.Context) ([]uuid.UUID, error) {
	args := m.Called(ctx)
	return args.Get(0).([]uuid.UUID), args.Error(1)
}
func (m *MockRetentionRepo) FindExpiredIncidents(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]database.ExpiredRecord, error) {
	args := m.Called(ctx, orgID, olderThanDays)
	return args.Get(0).([]database.ExpiredRecord), args.Error(1)
}
func (m *MockRetentionRepo) FindExpiredEvidence(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]database.ExpiredRecord, error) {
	args := m.Called(ctx, orgID, olderThanDays)
	return args.Get(0).([]database.ExpiredRecord), args.Error(1)
}
func (m *MockRetentionRepo) FindExpiredAgentRuns(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]database.ExpiredRecord, error) {
	args := m.Called(ctx, orgID, olderThanDays)
	return args.Get(0).([]database.ExpiredRecord), args.Error(1)
}
func (m *MockRetentionRepo) ArchiveRecord(ctx context.Context, orgID uuid.UUID, resourceType string, resourceID uuid.UUID, data json.RawMessage) error {
	return m.Called(ctx, orgID, resourceType, resourceID, data).Error(0)
}
func (m *MockRetentionRepo) DeleteIncidentCascade(ctx context.Context, incidentID uuid.UUID) error {
	return m.Called(ctx, incidentID).Error(0)
}
func (m *MockRetentionRepo) DeleteEvidence(ctx context.Context, evidenceID uuid.UUID) error {
	return m.Called(ctx, evidenceID).Error(0)
}
func (m *MockRetentionRepo) DeleteAgentRun(ctx context.Context, agentRunID uuid.UUID) error {
	return m.Called(ctx, agentRunID).Error(0)
}

// --- Tests ---

func TestRetentionService_CreatePolicy_DefaultsEnabledTrue(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	orgID := uuid.New()

	repo.On("CreatePolicy", mock.Anything, mock.MatchedBy(func(p *models.RetentionPolicy) bool {
		return p.OrgID == orgID && p.Enabled == true && p.ResourceType == models.RetentionResourceEvidence
	})).Return(nil)

	policy, err := svc.CreatePolicy(context.Background(), orgID, models.CreateRetentionPolicyRequest{
		ResourceType:  models.RetentionResourceEvidence,
		RetentionDays: 90,
		Action:        models.RetentionActionArchive,
	})

	require.NoError(t, err)
	assert.True(t, policy.Enabled)
	assert.Equal(t, 90, policy.RetentionDays)
	repo.AssertExpectations(t)
}

func TestRetentionService_UpdatePolicy_PartialFields(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	id := uuid.New()
	orgID := uuid.New()

	existing := &models.RetentionPolicy{
		ID: id, OrgID: orgID, ResourceType: models.RetentionResourceIncidents,
		RetentionDays: 365, Action: models.RetentionActionArchive, Enabled: true,
	}
	repo.On("GetPolicy", mock.Anything, id).Return(existing, nil)
	repo.On("UpdatePolicy", mock.Anything, mock.MatchedBy(func(p *models.RetentionPolicy) bool {
		return p.RetentionDays == 180 && p.Action == models.RetentionActionArchive && p.Enabled == true
	})).Return(nil)

	newDays := 180
	updated, err := svc.UpdatePolicy(context.Background(), id, models.UpdateRetentionPolicyRequest{
		RetentionDays: &newDays,
	})

	require.NoError(t, err)
	assert.Equal(t, 180, updated.RetentionDays)
	repo.AssertExpectations(t)
}

func TestRunRetentionSweep_ArchivesThenDeletes(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	orgID := uuid.New()
	policyID := uuid.New()
	recID := uuid.New()

	policies := []models.RetentionPolicy{
		{ID: policyID, OrgID: orgID, ResourceType: models.RetentionResourceIncidents, RetentionDays: 365, Action: models.RetentionActionArchive, Enabled: true},
	}
	expired := []database.ExpiredRecord{{ID: recID, Data: json.RawMessage(`{"id":"` + recID.String() + `"}`)}}

	repo.On("ListEnabledPolicies", mock.Anything, orgID).Return(policies, nil)
	repo.On("FindExpiredIncidents", mock.Anything, orgID, 365).Return(expired, nil)
	repo.On("ArchiveRecord", mock.Anything, orgID, models.RetentionResourceIncidents, recID, expired[0].Data).Return(nil)
	repo.On("DeleteIncidentCascade", mock.Anything, recID).Return(nil)

	summary, err := svc.RunRetentionSweep(context.Background(), orgID)

	require.NoError(t, err)
	require.Len(t, summary.Results, 1)
	assert.Equal(t, 1, summary.Results[0].MatchedCount)
	assert.Equal(t, 1, summary.Results[0].ArchivedCount)
	assert.Equal(t, 1, summary.Results[0].DeletedCount)
	assert.Empty(t, summary.Results[0].Errors)
	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "DeleteEvidence", mock.Anything, mock.Anything)
}

func TestRunRetentionSweep_DeleteAction_SkipsArchive(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	orgID := uuid.New()
	policyID := uuid.New()
	recID := uuid.New()

	policies := []models.RetentionPolicy{
		{ID: policyID, OrgID: orgID, ResourceType: models.RetentionResourceEvidence, RetentionDays: 90, Action: models.RetentionActionDelete, Enabled: true},
	}
	expired := []database.ExpiredRecord{{ID: recID, Data: json.RawMessage(`{}`)}}

	repo.On("ListEnabledPolicies", mock.Anything, orgID).Return(policies, nil)
	repo.On("FindExpiredEvidence", mock.Anything, orgID, 90).Return(expired, nil)
	repo.On("DeleteEvidence", mock.Anything, recID).Return(nil)

	summary, err := svc.RunRetentionSweep(context.Background(), orgID)

	require.NoError(t, err)
	require.Len(t, summary.Results, 1)
	assert.Equal(t, 0, summary.Results[0].ArchivedCount)
	assert.Equal(t, 1, summary.Results[0].DeletedCount)
	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "ArchiveRecord", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestRunRetentionSweep_NoExpiredRecords(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	orgID := uuid.New()
	policyID := uuid.New()

	policies := []models.RetentionPolicy{
		{ID: policyID, OrgID: orgID, ResourceType: models.RetentionResourceAgentRuns, RetentionDays: 30, Action: models.RetentionActionDelete, Enabled: true},
	}
	repo.On("ListEnabledPolicies", mock.Anything, orgID).Return(policies, nil)
	repo.On("FindExpiredAgentRuns", mock.Anything, orgID, 30).Return([]database.ExpiredRecord{}, nil)

	summary, err := svc.RunRetentionSweep(context.Background(), orgID)

	require.NoError(t, err)
	require.Len(t, summary.Results, 1)
	assert.Equal(t, 0, summary.Results[0].MatchedCount)
	repo.AssertNotCalled(t, "DeleteAgentRun", mock.Anything, mock.Anything)
}

func TestRunRetentionSweep_NoEnabledPolicies_EmptyResults(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	orgID := uuid.New()

	repo.On("ListEnabledPolicies", mock.Anything, orgID).Return([]models.RetentionPolicy{}, nil)

	summary, err := svc.RunRetentionSweep(context.Background(), orgID)

	require.NoError(t, err)
	assert.Empty(t, summary.Results)
}

func TestRunRetentionSweep_ArchiveErrorDoesNotDelete(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	orgID := uuid.New()
	policyID := uuid.New()
	recID := uuid.New()

	policies := []models.RetentionPolicy{
		{ID: policyID, OrgID: orgID, ResourceType: models.RetentionResourceIncidents, RetentionDays: 365, Action: models.RetentionActionArchive, Enabled: true},
	}
	expired := []database.ExpiredRecord{{ID: recID, Data: json.RawMessage(`{}`)}}

	repo.On("ListEnabledPolicies", mock.Anything, orgID).Return(policies, nil)
	repo.On("FindExpiredIncidents", mock.Anything, orgID, 365).Return(expired, nil)
	repo.On("ArchiveRecord", mock.Anything, orgID, models.RetentionResourceIncidents, recID, expired[0].Data).
		Return(assert.AnError)

	summary, err := svc.RunRetentionSweep(context.Background(), orgID)

	require.NoError(t, err)
	require.Len(t, summary.Results, 1)
	assert.Equal(t, 0, summary.Results[0].DeletedCount)
	assert.NotEmpty(t, summary.Results[0].Errors)
	repo.AssertNotCalled(t, "DeleteIncidentCascade", mock.Anything, mock.Anything)
}

func TestRunAllOrgsSweep_IteratesEveryOrg(t *testing.T) {
	repo := new(MockRetentionRepo)
	svc := NewRetentionService(repo)
	org1, org2 := uuid.New(), uuid.New()

	repo.On("ListOrgIDs", mock.Anything).Return([]uuid.UUID{org1, org2}, nil)
	repo.On("ListEnabledPolicies", mock.Anything, org1).Return([]models.RetentionPolicy{}, nil)
	repo.On("ListEnabledPolicies", mock.Anything, org2).Return([]models.RetentionPolicy{}, nil)

	summaries, err := svc.RunAllOrgsSweep(context.Background())

	require.NoError(t, err)
	assert.Len(t, summaries, 2)
	repo.AssertExpectations(t)
}
