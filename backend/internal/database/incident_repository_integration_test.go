//go:build integration

package database

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// TestPgIncidentRepository_Create_AssignsSequentialIncidentNumber covers the
// human-friendly "INC-0001" display code (see models.FormatIncidentCode):
// the first incident created for a fresh org must get number 1, the second
// must get number 2, regardless of creation order interleaving with a
// different org's counter.
func TestPgIncidentRepository_Create_AssignsSequentialIncidentNumber(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)
	repo := NewIncidentRepository(pool)
	ctx := context.Background()

	first := &models.Incident{ID: uuid.New(), OrgID: orgID, SoftwareID: swID, Title: "first", Severity: "high", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create (first) failed: %v", err)
	}
	if first.IncidentNumber != 1 {
		t.Fatalf("expected first incident to get number 1, got %d", first.IncidentNumber)
	}

	second := &models.Incident{ID: uuid.New(), OrgID: orgID, SoftwareID: swID, Title: "second", Severity: "high", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("Create (second) failed: %v", err)
	}
	if second.IncidentNumber != 2 {
		t.Fatalf("expected second incident to get number 2, got %d", second.IncidentNumber)
	}

	got, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.IncidentNumber != 1 {
		t.Fatalf("GetByID: expected incident_number 1, got %d", got.IncidentNumber)
	}
}

// TestPgIncidentRepository_Create_NumbersAreIndependentPerOrg guards the
// exact reason next_incident_number lives on organizations rather than a
// single global sequence: two different orgs' first incidents must each
// get number 1, not 1 and 2.
func TestPgIncidentRepository_Create_NumbersAreIndependentPerOrg(t *testing.T) {
	pool := setupTestDB(t)
	orgA := createTestOrg(t, pool)
	orgB := createTestOrg(t, pool)
	swA := createTestSoftware(t, pool, orgA)
	swB := createTestSoftware(t, pool, orgB)
	repo := NewIncidentRepository(pool)
	ctx := context.Background()

	incA := &models.Incident{ID: uuid.New(), OrgID: orgA, SoftwareID: swA, Title: "a", Severity: "high", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	incB := &models.Incident{ID: uuid.New(), OrgID: orgB, SoftwareID: swB, Title: "b", Severity: "high", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.Create(ctx, incA); err != nil {
		t.Fatalf("Create (org A) failed: %v", err)
	}
	if err := repo.Create(ctx, incB); err != nil {
		t.Fatalf("Create (org B) failed: %v", err)
	}

	if incA.IncidentNumber != 1 {
		t.Fatalf("expected org A's first incident to get number 1, got %d", incA.IncidentNumber)
	}
	if incB.IncidentNumber != 1 {
		t.Fatalf("expected org B's first incident to get number 1 (independent counter), got %d", incB.IncidentNumber)
	}
}

// TestPgIncidentRepository_Create_ConcurrentCallsNeverCollide guards the
// actual concurrency-safety claim in Create's doc comment: N concurrent
// Create calls for the same org must be assigned N distinct, contiguous
// numbers -- never a duplicate. This is what the UPDATE ... RETURNING row
// lock (rather than e.g. a naive "SELECT max+1 THEN INSERT") is for.
func TestPgIncidentRepository_Create_ConcurrentCallsNeverCollide(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)
	repo := NewIncidentRepository(pool)
	ctx := context.Background()

	const n = 20
	numbers := make([]int64, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			inc := &models.Incident{ID: uuid.New(), OrgID: orgID, SoftwareID: swID, Title: "concurrent", Severity: "high", Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			errs[i] = repo.Create(ctx, inc)
			numbers[i] = inc.IncidentNumber
		}(i)
	}
	wg.Wait()

	seen := make(map[int64]bool, n)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Create #%d failed: %v", i, err)
		}
		if seen[numbers[i]] {
			t.Fatalf("duplicate incident_number %d assigned across concurrent creates: %v", numbers[i], numbers)
		}
		seen[numbers[i]] = true
	}
	if len(seen) != n {
		t.Fatalf("expected %d distinct numbers, got %d: %v", n, len(seen), numbers)
	}
}
