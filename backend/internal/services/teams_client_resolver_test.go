package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockTeamsSettingsReader struct{ mock.Mock }

func (m *MockTeamsSettingsReader) GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error) {
	args := m.Called(ctx, orgID)
	s, _ := args.Get(0).(database.TeamsSettings)
	return s, args.Error(1)
}

type MockTeamsRefreshTokenUpdater struct{ mock.Mock }

func (m *MockTeamsRefreshTokenUpdater) UpdateTeamsRefreshToken(ctx context.Context, orgID uuid.UUID, newRefreshToken string) error {
	args := m.Called(ctx, orgID, newRefreshToken)
	return args.Error(0)
}

func TestTeamsClientResolver_Configured_ReturnsGraphClient(t *testing.T) {
	settings := new(MockTeamsSettingsReader)
	tokens := new(MockTeamsRefreshTokenUpdater)
	orgID := uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "t", ClientID: "c", ClientSecret: "s", RefreshToken: "r",
	}, nil)

	resolve := NewTeamsClientResolver(settings, tokens, false)
	client, err := resolve(context.Background(), orgID)

	require.NoError(t, err)
	_, isGraph := client.(*teams.GraphTeamsClient)
	require.True(t, isGraph, "expected a real, delegated GraphTeamsClient when configured")
}

func TestTeamsClientResolver_NotConfigured_MockMode_ReturnsMockClient(t *testing.T) {
	settings := new(MockTeamsSettingsReader)
	tokens := new(MockTeamsRefreshTokenUpdater)
	orgID := uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{}, nil)

	resolve := NewTeamsClientResolver(settings, tokens, true)
	client, err := resolve(context.Background(), orgID)

	require.NoError(t, err)
	_, isMock := client.(*teams.MockTeamsClient)
	require.True(t, isMock, "expected MockTeamsClient when unconfigured and mockMode=true")
}

func TestTeamsClientResolver_NotConfigured_NoMockMode_ReturnsNoopClient(t *testing.T) {
	settings := new(MockTeamsSettingsReader)
	tokens := new(MockTeamsRefreshTokenUpdater)
	orgID := uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{}, nil)

	resolve := NewTeamsClientResolver(settings, tokens, false)
	client, err := resolve(context.Background(), orgID)

	require.NoError(t, err)
	_, isNoop := client.(*teams.NoopTeamsClient)
	require.True(t, isNoop, "expected NoopTeamsClient when unconfigured and mockMode=false")
}

func TestTeamsClientResolver_PartiallyConfigured_FallsBackLikeUnconfigured(t *testing.T) {
	settings := new(MockTeamsSettingsReader)
	tokens := new(MockTeamsRefreshTokenUpdater)
	orgID := uuid.New()
	// tenant/client/secret set but no refresh token yet (never connected an
	// account) -- must not be treated as configured.
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "t", ClientID: "c", ClientSecret: "s",
	}, nil)

	resolve := NewTeamsClientResolver(settings, tokens, false)
	client, err := resolve(context.Background(), orgID)

	require.NoError(t, err)
	_, isNoop := client.(*teams.NoopTeamsClient)
	require.True(t, isNoop)
}

func TestTeamsClientResolver_SettingsReadError_Propagates(t *testing.T) {
	settings := new(MockTeamsSettingsReader)
	tokens := new(MockTeamsRefreshTokenUpdater)
	orgID := uuid.New()
	wantErr := errors.New("db unavailable")
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{}, wantErr)

	resolve := NewTeamsClientResolver(settings, tokens, false)
	_, err := resolve(context.Background(), orgID)

	require.ErrorIs(t, err, wantErr)
}

// TestTeamsClientResolver_DifferentOrgsGetDifferentSettings is the crux of
// the whole feature: two orgs with different Teams config must each
// resolve their own client, not share one process-wide instance the way
// teams.NewClientFromEnv() used to.
func TestTeamsClientResolver_DifferentOrgsGetDifferentSettings(t *testing.T) {
	settings := new(MockTeamsSettingsReader)
	tokens := new(MockTeamsRefreshTokenUpdater)
	orgA, orgB := uuid.New(), uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgA).Return(database.TeamsSettings{
		TenantID: "t", ClientID: "c", ClientSecret: "s", RefreshToken: "r",
	}, nil)
	settings.On("GetOrgTeamsSettings", mock.Anything, orgB).Return(database.TeamsSettings{}, nil)

	resolve := NewTeamsClientResolver(settings, tokens, false)

	clientA, err := resolve(context.Background(), orgA)
	require.NoError(t, err)
	clientB, err := resolve(context.Background(), orgB)
	require.NoError(t, err)

	_, aIsGraph := clientA.(*teams.GraphTeamsClient)
	_, bIsNoop := clientB.(*teams.NoopTeamsClient)
	require.True(t, aIsGraph)
	require.True(t, bIsNoop)
}
