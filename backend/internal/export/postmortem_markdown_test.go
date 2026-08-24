package export

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func samplePostmortem() *models.IncidentPostmortem {
	actionItems, _ := json.Marshal([]actionItem{
		{Title: "Add canary check", Description: "Prevent bad deploys", Priority: "high", Assignee: "alice"},
		{Title: "Update runbook", Priority: "low"},
	})
	lessons, _ := json.Marshal([]string{"Alerting was too slow", "Runbook was outdated"})
	wentWell, _ := json.Marshal([]string{"Team responded quickly"})
	wentWrong, _ := json.Marshal([]string{"No canary deploy"})
	prevention, _ := json.Marshal([]string{"Add canary deploys", "Improve alerting"})

	return &models.IncidentPostmortem{
		ID:                        uuid.New(),
		IncidentID:                uuid.New(),
		Status:                    "published",
		Title:                     "Checkout Service Outage",
		ExecutiveSummary:          "A deploy caused a 30 minute outage in checkout.",
		IncidentTimelineNarrative: "14:00 deploy started. 14:05 errors spiked. 14:30 rolled back.",
		RootCauseDetail:           "A null pointer dereference in the new payment client.",
		ImpactDetail:              "5% of checkout requests failed for 30 minutes.",
		LessonsLearned:            lessons,
		ActionItems:               actionItems,
		WhatWentWell:              wentWell,
		WhatWentWrong:             wentWrong,
		PreventionMeasures:        prevention,
		CreatedAt:                 time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
}

func sampleIncident(id uuid.UUID) *models.Incident {
	return &models.Incident{
		ID:       id,
		Title:    "Checkout errors spike",
		Severity: "sev1",
		Status:   "resolved",
	}
}

func TestRenderMarkdown_SectionsAndContent(t *testing.T) {
	pm := samplePostmortem()
	incident := sampleIncident(pm.IncidentID)

	md, err := RenderMarkdown(pm, incident)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(md, "# Checkout Service Outage\n"))

	for _, heading := range []string{
		"## Executive Summary",
		"## Incident Timeline",
		"## Root Cause",
		"## Impact",
		"## Lessons Learned",
		"## What Went Well",
		"## What Went Wrong",
		"## Action Items",
		"## Prevention Measures",
	} {
		assert.Contains(t, md, heading, "missing section: %s", heading)
	}

	assert.Contains(t, md, "A deploy caused a 30 minute outage in checkout.")
	assert.Contains(t, md, "A null pointer dereference in the new payment client.")
	assert.Contains(t, md, "- Alerting was too slow")
	assert.Contains(t, md, "- Team responded quickly")
	assert.Contains(t, md, "- No canary deploy")
	assert.Contains(t, md, "- Add canary deploys")
	assert.Contains(t, md, "Add canary check")
	assert.Contains(t, md, "alice")
	assert.Contains(t, md, "high")
	assert.Contains(t, md, "**Incident:** Checkout errors spike")
	assert.Contains(t, md, "**Severity:** sev1")
}

func TestRenderMarkdown_NilIncidentDoesNotPanic(t *testing.T) {
	pm := samplePostmortem()
	md, err := RenderMarkdown(pm, nil)
	require.NoError(t, err)
	assert.Contains(t, md, pm.IncidentID.String())
}

func TestRenderMarkdown_EmptyOptionalFields(t *testing.T) {
	pm := &models.IncidentPostmortem{
		ID:         uuid.New(),
		IncidentID: uuid.New(),
		Title:      "",
	}

	var md string
	var err error
	assert.NotPanics(t, func() {
		md, err = RenderMarkdown(pm, nil)
	})
	require.NoError(t, err)

	assert.Contains(t, md, "# Postmortem")
	assert.Contains(t, md, "_Not documented._")
	assert.Contains(t, md, "_None recorded._")
}

func TestRenderMarkdown_NilPostmortemErrors(t *testing.T) {
	_, err := RenderMarkdown(nil, nil)
	assert.Error(t, err)
}

func TestRenderMarkdown_MalformedJSONFieldsDoNotPanic(t *testing.T) {
	pm := &models.IncidentPostmortem{
		ID:             uuid.New(),
		IncidentID:     uuid.New(),
		Title:          "Bad JSON",
		LessonsLearned: json.RawMessage(`{"not": "an array"}`),
		ActionItems:    json.RawMessage(`not even json`),
	}

	var md string
	var err error
	assert.NotPanics(t, func() {
		md, err = RenderMarkdown(pm, nil)
	})
	require.NoError(t, err)
	assert.Contains(t, md, "## Lessons Learned")
	assert.Contains(t, md, "## Action Items")
}
