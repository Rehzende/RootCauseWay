package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSoftwareRepo is a mock for SoftwareRepository
type MockSoftwareRepo struct {
	mock.Mock
}

func (m *MockSoftwareRepo) Create(ctx context.Context, entry *models.SoftwareEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockSoftwareRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SoftwareEntry), args.Error(1)
}

func (m *MockSoftwareRepo) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SoftwareEntry, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	return args.Get(0).([]models.SoftwareEntry), args.Int(1), args.Error(2)
}

func (m *MockSoftwareRepo) Update(ctx context.Context, entry *models.SoftwareEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockSoftwareRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSoftwareRepo) FindBySlugOrTag(ctx context.Context, orgID uuid.UUID, label string) (*models.SoftwareEntry, error) {
	args := m.Called(ctx, orgID, label)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SoftwareEntry), args.Error(1)
}

func (m *MockSoftwareRepo) ListDependents(ctx context.Context, orgID uuid.UUID, slug string) ([]models.SoftwareEntry, error) {
	args := m.Called(ctx, orgID, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.SoftwareEntry), args.Error(1)
}

func TestSoftwareService_Create(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	orgID := uuid.New()
	req := models.CreateSoftwareRequest{
		Name: "API Service",
		Slug: "api-service",
		Tags: []string{"go", "api"},
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	result, err := svc.Create(context.Background(), orgID, req)

	require.NoError(t, err)
	assert.Equal(t, "API Service", result.Name)
	assert.Equal(t, "api-service", result.Slug)
	assert.Equal(t, orgID, result.OrgID)
	assert.Equal(t, "active", result.Status)
	repo.AssertExpectations(t)
}

func TestSoftwareService_Create_NilTags(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	req := models.CreateSoftwareRequest{
		Name: "Test",
		Slug: "test",
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	result, err := svc.Create(context.Background(), uuid.New(), req)

	require.NoError(t, err)
	assert.NotNil(t, result.Tags)
	assert.Empty(t, result.Tags)
}

func TestSoftwareService_GetByID(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	id := uuid.New()
	expected := &models.SoftwareEntry{ID: id, Name: "Test"}

	repo.On("GetByID", mock.Anything, id).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), id)

	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestSoftwareService_List_DefaultPagination(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	orgID := uuid.New()
	entries := []models.SoftwareEntry{{Name: "A"}, {Name: "B"}}

	repo.On("List", mock.Anything, orgID, 1, 20).Return(entries, 2, nil)

	result, total, err := svc.List(context.Background(), orgID, 0, 0)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
}

func TestSoftwareService_Update(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	id := uuid.New()
	existing := &models.SoftwareEntry{ID: id, Name: "Old", Slug: "old"}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	req := models.CreateSoftwareRequest{Name: "New", Slug: "new", Tags: []string{"updated"}}
	result, err := svc.Update(context.Background(), id, req)

	require.NoError(t, err)
	assert.Equal(t, "New", result.Name)
	assert.Equal(t, "new", result.Slug)
}

func TestSoftwareService_Delete(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	id := uuid.New()
	repo.On("Delete", mock.Anything, id).Return(nil)

	err := svc.Delete(context.Background(), id)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestSoftwareService_GetDependencyGraph_UpstreamAndDownstream(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	orgID := uuid.New()
	id := uuid.New()
	dbID := uuid.New()
	consumerID := uuid.New()

	entry := &models.SoftwareEntry{
		ID: id, OrgID: orgID, Name: "api-service", Slug: "api-service",
		Dependencies: json.RawMessage(`["postgres-primary"]`),
	}
	dbEntry := &models.SoftwareEntry{ID: dbID, OrgID: orgID, Name: "Postgres Primary", Slug: "postgres-primary"}
	dependents := []models.SoftwareEntry{
		{ID: consumerID, OrgID: orgID, Name: "checkout-service", Slug: "checkout-service"},
	}

	repo.On("GetByID", mock.Anything, id).Return(entry, nil)
	repo.On("FindBySlugOrTag", mock.Anything, orgID, "postgres-primary").Return(dbEntry, nil)
	repo.On("ListDependents", mock.Anything, orgID, "api-service").Return(dependents, nil)

	graph, err := svc.GetDependencyGraph(context.Background(), id)

	require.NoError(t, err)
	require.Len(t, graph.Upstream, 1)
	assert.Equal(t, dbID, graph.Upstream[0].SoftwareID)
	assert.Equal(t, "postgres-primary", graph.Upstream[0].Slug)
	require.Len(t, graph.Downstream, 1)
	assert.Equal(t, consumerID, graph.Downstream[0].SoftwareID)
	assert.Equal(t, "checkout-service", graph.Downstream[0].Slug)
}

func TestSoftwareService_GetDependencyGraph_SkipsUnresolvedUpstream(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	orgID := uuid.New()
	id := uuid.New()

	entry := &models.SoftwareEntry{
		ID: id, OrgID: orgID, Name: "api-service", Slug: "api-service",
		Dependencies: json.RawMessage(`["external-saas"]`),
	}

	repo.On("GetByID", mock.Anything, id).Return(entry, nil)
	repo.On("FindBySlugOrTag", mock.Anything, orgID, "external-saas").Return(nil, assert.AnError)
	repo.On("ListDependents", mock.Anything, orgID, "api-service").Return([]models.SoftwareEntry{}, nil)

	graph, err := svc.GetDependencyGraph(context.Background(), id)

	require.NoError(t, err)
	assert.Empty(t, graph.Upstream)
	assert.Empty(t, graph.Downstream)
}

func TestSoftwareService_GetDependencyGraph_NotFound(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, assert.AnError)

	graph, err := svc.GetDependencyGraph(context.Background(), id)

	require.Error(t, err)
	assert.Nil(t, graph)
}
