//go:build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// createTestIncidentWithTimes inserts a minimal incident row for the given
// software with an explicit created_at/resolved_at, bypassing the normal
// service layer so downtime windows are fully controlled by the test.
func createTestIncidentWithTimes(t *testing.T, pool *pgxpool.Pool, orgID, softwareID uuid.UUID, createdAt time.Time, resolvedAt *time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO incidents (id, org_id, software_id, title, severity, status, created_at, resolved_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, orgID, softwareID, "Test incident", "high",
		map[bool]string{true: "resolved", false: "open"}[resolvedAt != nil],
		createdAt, resolvedAt)
	if err != nil {
		t.Fatalf("failed to create test incident: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM incidents WHERE id=$1`, id)
	})
	return id
}

func TestSLORepository_CreateGetListUpdateDelete(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)

	repo := NewSLORepository(pool)

	now := time.Now().UTC().Truncate(time.Millisecond)
	def := &models.SLODefinition{
		ID:                    uuid.New(),
		OrgID:                 orgID,
		SoftwareID:            swID,
		Name:                  "API availability",
		SLOType:               models.SLOTypeAvailability,
		TargetPercentage:      99.9,
		MeasurementWindowDays: 30,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := repo.Create(ctx, def); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM slo_definitions WHERE id=$1`, def.ID)
	})

	got, err := repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != def.Name || got.TargetPercentage != def.TargetPercentage {
		t.Fatalf("GetByID mismatch: got %+v, want %+v", got, def)
	}

	list, err := repo.List(ctx, orgID)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, d := range list {
		if d.ID == def.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("List did not include created definition")
	}

	bySW, err := repo.ListBySoftware(ctx, swID)
	if err != nil {
		t.Fatalf("ListBySoftware failed: %v", err)
	}
	if len(bySW) != 1 || bySW[0].ID != def.ID {
		t.Fatalf("ListBySoftware mismatch: %+v", bySW)
	}

	def.TargetPercentage = 99.95
	def.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, def); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, err = repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("GetByID after update failed: %v", err)
	}
	if got.TargetPercentage != 99.95 {
		t.Fatalf("Update did not persist: got %v", got.TargetPercentage)
	}

	if err := repo.Delete(ctx, def.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.GetByID(ctx, def.ID); err == nil {
		t.Fatalf("expected error after delete, got nil")
	}
}

func TestSLORepository_GetIncidentDowntimeMinutes(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)
	repo := NewSLORepository(pool)

	windowEnd := time.Now().UTC()
	windowStart := windowEnd.Add(-24 * time.Hour)

	// Incident fully inside the window: 30 minutes downtime.
	c1 := windowStart.Add(1 * time.Hour)
	r1 := c1.Add(30 * time.Minute)
	createTestIncidentWithTimes(t, pool, orgID, swID, c1, &r1)

	// Incident starting before the window and resolved inside it: only the
	// portion inside the window (10 minutes) should count.
	c2 := windowStart.Add(-1 * time.Hour)
	r2 := windowStart.Add(10 * time.Minute)
	createTestIncidentWithTimes(t, pool, orgID, swID, c2, &r2)

	// Incident entirely outside the window: should not count at all.
	c3 := windowStart.Add(-5 * time.Hour)
	r3 := windowStart.Add(-4 * time.Hour)
	createTestIncidentWithTimes(t, pool, orgID, swID, c3, &r3)

	downtimeMinutes, incidentCount, err := repo.GetIncidentDowntimeMinutes(ctx, orgID, swID, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GetIncidentDowntimeMinutes failed: %v", err)
	}
	if incidentCount != 2 {
		t.Fatalf("expected 2 overlapping incidents, got %d", incidentCount)
	}
	want := 40.0 // 30 + 10 minutes
	if downtimeMinutes < want-0.01 || downtimeMinutes > want+0.01 {
		t.Fatalf("expected ~%.2f downtime minutes, got %.2f", want, downtimeMinutes)
	}
}
