package export

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPDF_ProducesValidPDFBytes(t *testing.T) {
	pm := samplePostmortem()
	incident := sampleIncident(pm.IncidentID)

	out, err := RenderPDF(pm, incident)
	require.NoError(t, err)
	require.NotEmpty(t, out)

	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")), "output should start with the PDF magic header")
	assert.Contains(t, string(out), "%%EOF")
}

func TestRenderPDF_NilIncidentDoesNotPanic(t *testing.T) {
	pm := samplePostmortem()

	var out []byte
	var err error
	assert.NotPanics(t, func() {
		out, err = RenderPDF(pm, nil)
	})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
}

func TestRenderPDF_EmptyOptionalFieldsDoNotPanic(t *testing.T) {
	pm := &models.IncidentPostmortem{
		ID:         uuid.New(),
		IncidentID: uuid.New(),
	}

	var out []byte
	var err error
	assert.NotPanics(t, func() {
		out, err = RenderPDF(pm, nil)
	})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
}

func TestRenderPDF_NilPostmortemErrors(t *testing.T) {
	_, err := RenderPDF(nil, nil)
	assert.Error(t, err)
}

func TestRenderPDF_MalformedJSONFieldsDoNotPanic(t *testing.T) {
	pm := &models.IncidentPostmortem{
		ID:           uuid.New(),
		IncidentID:   uuid.New(),
		ActionItems:  []byte(`not even json`),
		WhatWentWell: []byte(`{"nope": true}`),
	}

	var out []byte
	var err error
	assert.NotPanics(t, func() {
		out, err = RenderPDF(pm, nil)
	})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
}
