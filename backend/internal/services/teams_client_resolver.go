package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
)

// TeamsSettingsReader is the minimal read surface NewTeamsClientResolver
// needs from org settings storage. Kept narrow (like WarRoomIncidentReader
// above) so this package depends on database.PgPipelineGateRepository's
// interface, not its full concrete type.
type TeamsSettingsReader interface {
	GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error)
}

// TeamsRefreshTokenUpdater is the narrow write surface the delegated Graph
// client needs: persisting a rotated refresh token (see
// teams.NewGraphTeamsClientDelegated) without touching any other Teams
// setting. Almost always the same concrete repository as
// TeamsSettingsReader, but kept as a separate interface since it's a
// distinct, narrower capability (rotation should never risk a
// read-modify-write clobber of a concurrent settings edit).
type TeamsRefreshTokenUpdater interface {
	UpdateTeamsRefreshToken(ctx context.Context, orgID uuid.UUID, newRefreshToken string) error
}

// NewTeamsClientResolver builds a TeamsClientResolver that looks up each
// org's Teams integration settings (see the Integrations settings UI) on
// every call, instead of a single client fixed at process boot from env
// vars. Fallback order mirrors what teams.NewClientFromEnv used to apply
// globally, just evaluated per org now:
//   - configured (tenant/client/secret + a connected account's refresh
//     token, from the OAuth connect flow) -> real, delegated GraphTeamsClient
//   - not configured AND mockMode -> MockTeamsClient (dev/test)
//   - otherwise -> NoopTeamsClient, which fails clearly on every call
//
// mockMode is still a single process-wide toggle (WARROOM_MOCK_MODE), not
// per-org -- it's a local-dev/demo escape hatch, not a tenant setting.
func NewTeamsClientResolver(settings TeamsSettingsReader, tokens TeamsRefreshTokenUpdater, mockMode bool) TeamsClientResolver {
	return func(ctx context.Context, orgID uuid.UUID) (teams.TeamsClient, error) {
		s, err := settings.GetOrgTeamsSettings(ctx, orgID)
		if err != nil {
			return nil, err
		}
		if s.Configured() {
			persist := func(refreshCtx context.Context, newRefreshToken string) error {
				return tokens.UpdateTeamsRefreshToken(refreshCtx, orgID, newRefreshToken)
			}
			return teams.NewGraphTeamsClientDelegated(s.TenantID, s.ClientID, s.ClientSecret, s.RefreshToken, persist), nil
		}
		if mockMode {
			return teams.NewMockTeamsClient(), nil
		}
		return teams.NewNoopTeamsClient(), nil
	}
}
