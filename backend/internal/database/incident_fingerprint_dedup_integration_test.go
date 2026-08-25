//go:build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// TestPgIncidentRepository_FindByFingerprint_MatchesRegardlessOfWindowWhenStillOpen
// pins the fix for a live-found bug: a flapping alert (fires, resolves,
// fires again) whose gaps exceed the recency window kept spawning a new
// incident every cycle instead of landing on the one still tracking it, as
// long as that incident was still open. The query must match on fingerprint
// AND (recent OR still open) -- recency alone is not enough once the prior
// incident hasn't been resolved.
func TestPgIncidentRepository_FindByFingerprint_MatchesRegardlessOfWindowWhenStillOpen(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)
	repo := NewIncidentRepository(pool)

	incident := &models.Incident{
		ID: uuid.New(), OrgID: orgID, SoftwareID: swID, Title: "flapping alert",
		Severity: "high", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, incident); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	fingerprint := "flapping-fp-" + uuid.New().String()
	oldSnapshotTime := time.Now().Add(-2 * time.Hour) // well outside any recency window
	if _, err := pool.Exec(ctx,
		`INSERT INTO alert_snapshots (id, incident_id, software_id, raw_payload, normalized, created_at)
		 VALUES ($1, $2, $3, '{}', $4, $5)`,
		uuid.New(), incident.ID, swID,
		map[string]any{"fingerprint": fingerprint}, oldSnapshotTime,
	); err != nil {
		t.Fatalf("insert alert_snapshot failed: %v", err)
	}

	// Recency window is tiny (1s) -- the 2h-old snapshot only matches
	// because the incident is still open, not because it's recent.
	found, err := repo.FindByFingerprint(ctx, orgID, fingerprint, time.Now().Add(-1*time.Second))
	if err != nil {
		t.Fatalf("FindByFingerprint failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected FindByFingerprint to match the still-open incident despite the old snapshot, got nil")
	}
	if found.ID != incident.ID {
		t.Fatalf("expected match on incident %s, got %s", incident.ID, found.ID)
	}
}

// TestPgIncidentRepository_FindByFingerprint_ResolvedIncidentRespectsWindow
// confirms the flip side: once the incident IS resolved, an old snapshot
// outside the window must NOT match -- the "always match if open" carve-out
// only applies while genuinely unresolved, otherwise a fingerprint could
// never be reused for a legitimately new occurrence of the same alert.
func TestPgIncidentRepository_FindByFingerprint_ResolvedIncidentRespectsWindow(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)
	repo := NewIncidentRepository(pool)

	incident := &models.Incident{
		ID: uuid.New(), OrgID: orgID, SoftwareID: swID, Title: "resolved alert",
		Severity: "high", Status: "resolved", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, incident); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	fingerprint := "resolved-fp-" + uuid.New().String()
	oldSnapshotTime := time.Now().Add(-2 * time.Hour)
	if _, err := pool.Exec(ctx,
		`INSERT INTO alert_snapshots (id, incident_id, software_id, raw_payload, normalized, created_at)
		 VALUES ($1, $2, $3, '{}', $4, $5)`,
		uuid.New(), incident.ID, swID,
		map[string]any{"fingerprint": fingerprint}, oldSnapshotTime,
	); err != nil {
		t.Fatalf("insert alert_snapshot failed: %v", err)
	}

	found, err := repo.FindByFingerprint(ctx, orgID, fingerprint, time.Now().Add(-1*time.Second))
	if err != nil {
		t.Fatalf("FindByFingerprint failed: %v", err)
	}
	if found != nil {
		t.Fatalf("expected no match for a resolved incident's old snapshot, got %s", found.ID)
	}
}
