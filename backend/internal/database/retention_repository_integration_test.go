//go:build integration

// Retention repository tests require a live Postgres (ROOTCAUSEWAY_TEST_DB_URL or
// the default test_rootcauseway DB -- see setupTestDB in
// skills_repositories_integration_test.go) and are gated behind the
// `integration` build tag, matching the convention already used by
// skills_repositories_integration_test.go and warroom_repositories_test.go.
// `go test ./internal/database/...` (no -tags) skips this file entirely;
// run `go test -tags integration ./internal/database/...` with a reachable
// test DB to exercise it.
package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

func createTestIncidentForRetention(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, status string, resolvedAt *time.Time) uuid.UUID {
	t.Helper()
	swID := createTestSoftware(t, pool, orgID)
	incidentID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO incidents (id, org_id, software_id, title, severity, status, resolved_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		incidentID, orgID, swID, "Test incident for retention", "high", status, resolvedAt)
	if err != nil {
		t.Fatalf("failed to create test incident: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM incidents WHERE id=$1`, incidentID)
	})
	return incidentID
}

func createTestEvidence(t *testing.T, pool *pgxpool.Pool, incidentID uuid.UUID, collectedAt time.Time) uuid.UUID {
	t.Helper()
	evidenceID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO incident_evidence (id, incident_id, type, title, content, collected_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		evidenceID, incidentID, "log", "Test evidence", json.RawMessage(`{"line":"boom"}`), collectedAt)
	if err != nil {
		t.Fatalf("failed to create test evidence: %v", err)
	}
	return evidenceID
}

func TestRetentionRepository_FindExpiredIncidents_FiltersByStatusAndCutoff(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	repo := NewRetentionRepository(pool)

	old := time.Now().AddDate(0, 0, -400)
	recent := time.Now().AddDate(0, 0, -10)

	expiredClosed := createTestIncidentForRetention(t, pool, orgID, "closed", &old)
	_ = createTestIncidentForRetention(t, pool, orgID, "resolved", &recent)  // too recent
	_ = createTestIncidentForRetention(t, pool, orgID, "open", &old)         // wrong status (resolved_at set anyway)
	_ = createTestIncidentForRetention(t, pool, orgID, "investigating", nil) // no resolved_at

	results, err := repo.FindExpiredIncidents(ctx, orgID, 365)
	if err != nil {
		t.Fatalf("FindExpiredIncidents error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 expired incident, got %d", len(results))
	}
	if results[0].ID != expiredClosed {
		t.Errorf("expected expired incident %s, got %s", expiredClosed, results[0].ID)
	}
}

func TestRetentionRepository_FindExpiredEvidence_IgnoresIncidentStatus(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	repo := NewRetentionRepository(pool)

	// Evidence retention is independent of incident status: an open
	// incident's old evidence should still be found.
	openIncident := createTestIncidentForRetention(t, pool, orgID, "open", nil)
	oldEvidence := createTestEvidence(t, pool, openIncident, time.Now().AddDate(0, 0, -120))
	recentEvidence := createTestEvidence(t, pool, openIncident, time.Now().AddDate(0, 0, -5))
	_ = recentEvidence

	results, err := repo.FindExpiredEvidence(ctx, orgID, 90)
	if err != nil {
		t.Fatalf("FindExpiredEvidence error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 expired evidence row, got %d", len(results))
	}
	if results[0].ID != oldEvidence {
		t.Errorf("expected expired evidence %s, got %s", oldEvidence, results[0].ID)
	}
}

func TestRetentionRepository_ArchiveThenDeleteIncidentCascade(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	repo := NewRetentionRepository(pool)

	old := time.Now().AddDate(0, 0, -400)
	incidentID := createTestIncidentForRetention(t, pool, orgID, "closed", &old)
	evidenceID := createTestEvidence(t, pool, incidentID, old)

	snapshot := json.RawMessage(`{"id":"` + incidentID.String() + `"}`)
	if err := repo.ArchiveRecord(ctx, orgID, models.RetentionResourceIncidents, incidentID, snapshot); err != nil {
		t.Fatalf("ArchiveRecord error: %v", err)
	}

	var archivedCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM archived_records WHERE resource_id=$1`, incidentID).Scan(&archivedCount); err != nil {
		t.Fatalf("count archived_records error: %v", err)
	}
	if archivedCount != 1 {
		t.Fatalf("expected 1 archived_records row, got %d", archivedCount)
	}

	if err := repo.DeleteIncidentCascade(ctx, incidentID); err != nil {
		t.Fatalf("DeleteIncidentCascade error: %v", err)
	}

	var incidentCount, evidenceCount int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM incidents WHERE id=$1`, incidentID).Scan(&incidentCount)
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM incident_evidence WHERE id=$1`, evidenceID).Scan(&evidenceCount)
	if incidentCount != 0 {
		t.Errorf("expected incident to be deleted, still present")
	}
	if evidenceCount != 0 {
		t.Errorf("expected evidence to cascade-delete with its incident, still present")
	}
}

func TestRetentionRepository_PolicyCRUD(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	repo := NewRetentionRepository(pool)

	now := time.Now().UTC().Truncate(time.Millisecond)
	p := &models.RetentionPolicy{
		ID: uuid.New(), OrgID: orgID, ResourceType: models.RetentionResourceEvidence,
		RetentionDays: 90, Action: models.RetentionActionArchive, Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreatePolicy(ctx, p); err != nil {
		t.Fatalf("CreatePolicy error: %v", err)
	}

	got, err := repo.GetPolicy(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetPolicy error: %v", err)
	}
	if got.RetentionDays != 90 {
		t.Errorf("expected retention_days=90, got %d", got.RetentionDays)
	}

	got.RetentionDays = 120
	got.UpdatedAt = time.Now().UTC()
	if err := repo.UpdatePolicy(ctx, got); err != nil {
		t.Fatalf("UpdatePolicy error: %v", err)
	}
	reGot, _ := repo.GetPolicy(ctx, p.ID)
	if reGot.RetentionDays != 120 {
		t.Errorf("expected updated retention_days=120, got %d", reGot.RetentionDays)
	}

	enabled, err := repo.ListEnabledPolicies(ctx, orgID)
	if err != nil {
		t.Fatalf("ListEnabledPolicies error: %v", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled policy, got %d", len(enabled))
	}

	if err := repo.DeletePolicy(ctx, p.ID); err != nil {
		t.Fatalf("DeletePolicy error: %v", err)
	}
	if _, err := repo.GetPolicy(ctx, p.ID); err == nil {
		t.Errorf("expected error fetching deleted policy, got nil")
	}
}
