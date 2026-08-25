package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/google/uuid"
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

// TestSoftwareService_Update_PreservesOwnerIDWhenRequestOmitsIt pins a real
// data-loss bug: the catalog UI's edit form has no owner picker and never
// sends owner_id at all, so unconditionally assigning req.OwnerID (always
// nil in that case) silently wiped any owner set some other way on every
// single edit made through the UI.
func TestSoftwareService_Update_PreservesOwnerIDWhenRequestOmitsIt(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	id := uuid.New()
	ownerID := uuid.New()
	existing := &models.SoftwareEntry{ID: id, Name: "Old", Slug: "old", OwnerID: &ownerID}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	req := models.CreateSoftwareRequest{Name: "New", Slug: "new"} // no OwnerID set
	result, err := svc.Update(context.Background(), id, req)

	require.NoError(t, err)
	require.NotNil(t, result.OwnerID)
	assert.Equal(t, ownerID, *result.OwnerID)
}

func TestSoftwareService_Update_OverwritesOwnerIDWhenRequestProvidesOne(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	id := uuid.New()
	oldOwner := uuid.New()
	newOwner := uuid.New()
	existing := &models.SoftwareEntry{ID: id, Name: "Old", Slug: "old", OwnerID: &oldOwner}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	req := models.CreateSoftwareRequest{Name: "New", Slug: "new", OwnerID: &newOwner}
	result, err := svc.Update(context.Background(), id, req)

	require.NoError(t, err)
	require.NotNil(t, result.OwnerID)
	assert.Equal(t, newOwner, *result.OwnerID)
}

func TestSoftwareService_Create_DefaultsCriticalityAndType(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	result, err := svc.Create(context.Background(), uuid.New(), models.CreateSoftwareRequest{Name: "T", Slug: "t"})

	require.NoError(t, err)
	assert.Equal(t, "medium", result.Criticality)
	assert.Equal(t, "service", result.Type)
}

func TestSoftwareService_Create_HonorsExplicitCriticalityAndType(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.SoftwareEntry")).Return(nil)

	req := models.CreateSoftwareRequest{Name: "T", Slug: "t", Criticality: "critical", Type: "database"}
	result, err := svc.Create(context.Background(), uuid.New(), req)

	require.NoError(t, err)
	assert.Equal(t, "critical", result.Criticality)
	assert.Equal(t, "database", result.Type)
}

func TestValidateSoftwareRequest(t *testing.T) {
	cases := []struct {
		name    string
		req     models.CreateSoftwareRequest
		wantErr bool
	}{
		{"empty is valid (defaults apply later)", models.CreateSoftwareRequest{}, false},
		{"valid criticality and type", models.CreateSoftwareRequest{Criticality: "high", Type: "job"}, false},
		{"invalid criticality", models.CreateSoftwareRequest{Criticality: "urgent"}, true},
		{"invalid type", models.CreateSoftwareRequest{Type: "microservice"}, true},
		{
			"valid dependency relation",
			models.CreateSoftwareRequest{Dependencies: json.RawMessage(`[{"slug":"db","relation":"shares_database_with"}]`)},
			false,
		},
		{
			"invalid dependency relation",
			models.CreateSoftwareRequest{Dependencies: json.RawMessage(`[{"slug":"db","relation":"talks_to"}]`)},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSoftwareRequest(tc.req)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
		Dependencies: json.RawMessage(`[{"slug": "postgres-primary", "relation": "depends_on"}]`),
	}
	dbEntry := &models.SoftwareEntry{ID: dbID, OrgID: orgID, Name: "Postgres Primary", Slug: "postgres-primary"}
	dependents := []models.SoftwareEntry{
		{
			ID: consumerID, OrgID: orgID, Name: "checkout-service", Slug: "checkout-service",
			Dependencies: json.RawMessage(`[{"slug": "api-service", "relation": "uses_api_of"}]`),
		},
	}

	repo.On("GetByID", mock.Anything, id).Return(entry, nil)
	repo.On("FindBySlugOrTag", mock.Anything, orgID, "postgres-primary").Return(dbEntry, nil)
	repo.On("ListDependents", mock.Anything, orgID, "api-service").Return(dependents, nil)

	graph, err := svc.GetDependencyGraph(context.Background(), id)

	require.NoError(t, err)
	require.Len(t, graph.Upstream, 1)
	assert.Equal(t, dbID, graph.Upstream[0].SoftwareID)
	assert.Equal(t, "postgres-primary", graph.Upstream[0].Slug)
	assert.Equal(t, "depends_on", graph.Upstream[0].Relation)
	require.Len(t, graph.Downstream, 1)
	assert.Equal(t, consumerID, graph.Downstream[0].SoftwareID)
	assert.Equal(t, "checkout-service", graph.Downstream[0].Slug)
	assert.Equal(t, "uses_api_of", graph.Downstream[0].Relation)
}

// TestSoftwareService_GetDependencyGraph_ParsesLegacyFlatStringDependencies
// pins backward compatibility with the pre-migration-031 shape (a bare JSON
// array of slug strings) -- any row somehow missed by the migration's
// backfill must still resolve instead of silently dropping the dependency.
func TestSoftwareService_GetDependencyGraph_ParsesLegacyFlatStringDependencies(t *testing.T) {
	repo := new(MockSoftwareRepo)
	svc := NewSoftwareService(repo)

	orgID := uuid.New()
	id := uuid.New()
	dbID := uuid.New()

	entry := &models.SoftwareEntry{
		ID: id, OrgID: orgID, Name: "api-service", Slug: "api-service",
		Dependencies: json.RawMessage(`["postgres-primary"]`),
	}
	dbEntry := &models.SoftwareEntry{ID: dbID, OrgID: orgID, Name: "Postgres Primary", Slug: "postgres-primary"}

	repo.On("GetByID", mock.Anything, id).Return(entry, nil)
	repo.On("FindBySlugOrTag", mock.Anything, orgID, "postgres-primary").Return(dbEntry, nil)
	repo.On("ListDependents", mock.Anything, orgID, "api-service").Return([]models.SoftwareEntry{}, nil)

	graph, err := svc.GetDependencyGraph(context.Background(), id)

	require.NoError(t, err)
	require.Len(t, graph.Upstream, 1)
	assert.Equal(t, "postgres-primary", graph.Upstream[0].Slug)
	assert.Equal(t, "depends_on", graph.Upstream[0].Relation, "legacy bare-string dependency should default to depends_on")
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
