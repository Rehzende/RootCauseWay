//go:build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authRequest creates a request with the JWT Bearer token.
func authRequest(method, path string, body []byte, token string) *http.Request {
	var req *http.Request
	if body != nil {
		req, _ = http.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// --- Full Incident Flow ---

func TestIntegration_FullIncidentFlow(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	// 1. Create software entry
	swBody, _ := json.Marshal(models.CreateSoftwareRequest{
		Name: "payment-service",
		Slug: "payment-service",
		Description: "Payment processing microservice",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/software", swBody, token))
	require.Equal(t, http.StatusCreated, w.Code, "create software: %s", w.Body.String())

	var sw models.SoftwareEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sw))
	assert.Equal(t, "payment-service", sw.Name)
	softwareID := sw.ID

	// 2. Create webhook for Datadog
	whBody, _ := json.Marshal(models.CreateWebhookRequest{
		Name:       "dd-prod",
		Source:     "datadog",
		SoftwareID: softwareID,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/webhooks", whBody, token))
	require.Equal(t, http.StatusCreated, w.Code, "create webhook: %s", w.Body.String())

	var wh models.Webhook
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wh))
	assert.NotEmpty(t, wh.EndpointToken)
	ingestToken := wh.EndpointToken

	// 3. Ingest alert (public endpoint, no auth)
	alertPayload := []byte(`{"alert_id":"dd-123","alert_title":"High CPU on payment-service","alert_type":"error","hostname":"web-1","priority":"critical","tags":"service:payment,env:prod"}`)
	w = httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ingest/"+ingestToken, bytes.NewReader(alertPayload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code, "ingest alert: %s", w.Body.String())

	var ingestResult map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ingestResult))
	incidentIDStr := ingestResult["incident_id"].(string)
	incidentID, err := uuid.Parse(incidentIDStr)
	require.NoError(t, err)

	// 4. List incidents - should contain the new incident
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/incidents", nil, token))
	require.Equal(t, http.StatusOK, w.Code)

	var listResp models.PaginatedResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	assert.GreaterOrEqual(t, listResp.Total, 1)

	// 5. Get incident by ID
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/incidents/"+incidentID.String(), nil, token))
	require.Equal(t, http.StatusOK, w.Code)

	var incident models.Incident
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &incident))
	assert.Equal(t, "High CPU on payment-service", incident.Title)
	assert.Equal(t, "critical", incident.Severity)

	// 6. Update incident status
	status := "investigating"
	updateBody, _ := json.Marshal(models.UpdateIncidentRequest{Status: &status})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("PATCH", "/api/v1/incidents/"+incidentID.String(), updateBody, token))
	require.Equal(t, http.StatusOK, w.Code)

	var updated models.Incident
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "investigating", updated.Status)

	// 7. Add event (comment) to incident
	eventBody, _ := json.Marshal(models.CreateEventRequest{
		Type: "comment",
		Data: json.RawMessage(`{"text":"Investigating CPU spike"}`),
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/events", eventBody, token))
	require.Equal(t, http.StatusCreated, w.Code)

	// 8. Get full incident view
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/incidents/"+incidentID.String()+"/full", nil, token))
	require.Equal(t, http.StatusOK, w.Code)

	var full models.IncidentFull
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &full))
	assert.Equal(t, incidentID, full.ID)
}

// --- Webhook Validation ---

func TestIntegration_WebhookValidation(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	// Invalid token -> 404
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ingest/nonexistent-token",
		bytes.NewReader([]byte(`{"alert_id":"1","alert_title":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Empty payload -> 400
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/ingest/some-token",
		bytes.NewReader([]byte(``)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Malformed JSON -> 400
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/ingest/some-token",
		bytes.NewReader([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Alert Normalization ---

func TestIntegration_AlertNormalization(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	sources := []struct {
		name    string
		payload string
		title   string
	}{
		{
			name:    "datadog",
			payload: `{"alert_id":"dd-1","alert_title":"High Memory","alert_type":"error","hostname":"web-2","priority":"high","tags":"service:api,env:prod"}`,
			title:   "High Memory",
		},
		{
			name:    "prometheus_alertmanager",
			payload: `{"alerts":[{"status":"firing","labels":{"alertname":"DiskFull","severity":"critical","service":"storage"},"annotations":{"summary":"Disk 95% full"},"startsAt":"2026-01-01T00:00:00Z"}]}`,
			title:   "DiskFull",
		},
		{
			name:    "grafana",
			payload: `{"title":"Network Latency","state":"alerting","message":"Latency above threshold","evalMatches":[],"tags":{"service":"gateway","severity":"high"}}`,
			title:   "Network Latency",
		},
		{
			name:    "otel",
			payload: `{"alert_name":"SlowQueries","severity":"medium","service_name":"db-proxy","description":"P99 > 1s","attributes":{"env":"prod"},"start_time":"2026-01-01T00:00:00Z"}`,
			title:   "SlowQueries",
		},
	}

	for _, src := range sources {
		t.Run(src.name, func(t *testing.T) {
			// Create software for this source
			swBody, _ := json.Marshal(models.CreateSoftwareRequest{
				Name: "svc-" + src.name, Slug: "svc-" + src.name,
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, authRequest("POST", "/api/v1/software", swBody, token))
			require.Equal(t, http.StatusCreated, w.Code)
			var sw models.SoftwareEntry
			json.Unmarshal(w.Body.Bytes(), &sw)

			// Create webhook
			whBody, _ := json.Marshal(models.CreateWebhookRequest{
				Name: "wh-" + src.name, Source: src.name, SoftwareID: sw.ID,
			})
			w = httptest.NewRecorder()
			r.ServeHTTP(w, authRequest("POST", "/api/v1/webhooks", whBody, token))
			require.Equal(t, http.StatusCreated, w.Code)
			var wh models.Webhook
			json.Unmarshal(w.Body.Bytes(), &wh)

			// Ingest
			w = httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/ingest/"+wh.EndpointToken,
				bytes.NewReader([]byte(src.payload)))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusAccepted, w.Code, "source=%s body=%s", src.name, w.Body.String())

			// Get incident and verify title
			var result map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &result)
			incID := result["incident_id"].(string)

			w = httptest.NewRecorder()
			r.ServeHTTP(w, authRequest("GET", "/api/v1/incidents/"+incID, nil, token))
			require.Equal(t, http.StatusOK, w.Code)

			var inc models.Incident
			json.Unmarshal(w.Body.Bytes(), &inc)
			assert.Equal(t, src.title, inc.Title, "source=%s", src.name)
		})
	}
}

// --- Software Catalog CRUD ---

func TestIntegration_SoftwareCatalogCRUD(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	// Create
	body, _ := json.Marshal(models.CreateSoftwareRequest{
		Name:          "crud-service",
		Slug:          "crud-service",
		Description:   "Test CRUD service",
		CloudProvider: "aws",
		CloudResources: json.RawMessage(`[{"type":"ec2","name":"web-fleet"}]`),
		Stakeholders:   json.RawMessage(`[{"name":"John","role":"owner"}]`),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/software", body, token))
	require.Equal(t, http.StatusCreated, w.Code)

	var created models.SoftwareEntry
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created.ID

	// Read
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/software/"+id.String(), nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var fetched models.SoftwareEntry
	json.Unmarshal(w.Body.Bytes(), &fetched)
	assert.Equal(t, "crud-service", fetched.Name)
	assert.Equal(t, "aws", fetched.CloudProvider)

	// Update
	updateBody, _ := json.Marshal(models.CreateSoftwareRequest{
		Name:        "crud-service",
		Slug:        "crud-service",
		Description: "Updated description",
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("PUT", "/api/v1/software/"+id.String(), updateBody, token))
	require.Equal(t, http.StatusOK, w.Code)
	var updatedSw models.SoftwareEntry
	json.Unmarshal(w.Body.Bytes(), &updatedSw)
	assert.Equal(t, "Updated description", updatedSw.Description)

	// Delete
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("DELETE", "/api/v1/software/"+id.String(), nil, token))
	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify deleted
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/software/"+id.String(), nil, token))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- A2A Agents CRUD ---

func TestIntegration_A2AAgentsCRUD(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	agentCard := json.RawMessage(`{"name":"triage","url":"http://localhost:8090","version":"0.1.0","skills":[]}`)

	// Create
	body, _ := json.Marshal(models.CreateA2AAgentRequest{
		Name:        "triage-agent",
		AgentType:   "triage",
		EndpointURL: "http://localhost:8090",
		AgentCard:   agentCard,
		Description: "Triage analysis agent",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/a2a/agents", body, token))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var agent models.A2AAgent
	json.Unmarshal(w.Body.Bytes(), &agent)
	agentID := agent.ID

	// List
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/a2a/agents", nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var listResp models.PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.GreaterOrEqual(t, listResp.Total, 1)

	// Get card
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/v1/a2a/agents/%s/card", agentID), nil, token))
	require.Equal(t, http.StatusOK, w.Code)

	// Update
	updateBody, _ := json.Marshal(models.CreateA2AAgentRequest{
		Name:        "triage-agent-v2",
		AgentType:   "triage",
		EndpointURL: "http://localhost:8090",
		Description: "Updated triage agent",
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("PUT", "/api/v1/a2a/agents/"+agentID.String(), updateBody, token))
	require.Equal(t, http.StatusOK, w.Code)

	// Delete
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("DELETE", "/api/v1/a2a/agents/"+agentID.String(), nil, token))
	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify deleted
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/a2a/agents/"+agentID.String(), nil, token))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Skills and Linking ---

func TestIntegration_SkillsAndLinking(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	// Create A2A agent first
	agentBody, _ := json.Marshal(models.CreateA2AAgentRequest{
		Name: "skill-test-agent", AgentType: "triage",
		EndpointURL: "http://localhost:8090",
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/a2a/agents", agentBody, token))
	require.Equal(t, http.StatusCreated, w.Code)
	var agent models.A2AAgent
	json.Unmarshal(w.Body.Bytes(), &agent)

	// Create skill
	skillBody, _ := json.Marshal(models.CreateSkillRequest{
		Name:     "log-analysis",
		Slug:     "log-analysis",
		Category: "investigation",
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/skills", skillBody, token))
	require.Equal(t, http.StatusCreated, w.Code)
	var skill models.Skill
	json.Unmarshal(w.Body.Bytes(), &skill)

	// Link skill to agent
	linkBody, _ := json.Marshal(models.CreateAgentSkillLinkRequest{
		SkillID:  skill.ID,
		Priority: 1,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", fmt.Sprintf("/api/v1/a2a/agents/%s/skills", agent.ID), linkBody, token))
	require.Equal(t, http.StatusCreated, w.Code)

	// List agent skills
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/v1/a2a/agents/%s/skills", agent.ID), nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var links []models.AgentSkillLink
	json.Unmarshal(w.Body.Bytes(), &links)
	assert.Len(t, links, 1)

	// Unlink
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/v1/a2a/agents/%s/skills/%s", agent.ID, skill.ID), nil, token))
	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify unlinked
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/v1/a2a/agents/%s/skills", agent.ID), nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &links)
	assert.Len(t, links, 0)
}

// --- Credential Leases ---

func TestIntegration_CredentialLeases(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	// Create software
	swBody, _ := json.Marshal(models.CreateSoftwareRequest{Name: "cred-svc", Slug: "cred-svc"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/software", swBody, token))
	require.Equal(t, http.StatusCreated, w.Code)
	var sw models.SoftwareEntry
	json.Unmarshal(w.Body.Bytes(), &sw)

	// Create credential provider
	provBody, _ := json.Marshal(models.CreateCredentialProviderRequest{
		Name:         "test-vault",
		ProviderType: "vault",
		Config:       json.RawMessage(`{"address":"http://vault:8200"}`),
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/credentials/providers", provBody, token))
	require.Equal(t, http.StatusCreated, w.Code)
	var prov models.CredentialProvider
	json.Unmarshal(w.Body.Bytes(), &prov)

	// Create resource credential
	rcBody, _ := json.Marshal(models.CreateResourceCredentialRequest{
		SoftwareID:     sw.ID,
		ResourceName:   "payments-db",
		ResourceType:   "database",
		ProviderID:     prov.ID,
		CredentialPath: "secret/data/payments-db",
		DefaultTTL:     300,
		MaxTTL:         3600,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", fmt.Sprintf("/api/v1/software/%s/credentials", sw.ID), rcBody, token))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var rc models.ResourceCredential
	json.Unmarshal(w.Body.Bytes(), &rc)

	// Create A2A agent for the lease
	agentBody, _ := json.Marshal(models.CreateA2AAgentRequest{
		Name: "lease-agent", AgentType: "evidence",
		EndpointURL: "http://localhost:8091",
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/a2a/agents", agentBody, token))
	require.Equal(t, http.StatusCreated, w.Code)
	var a2aAgent models.A2AAgent
	json.Unmarshal(w.Body.Bytes(), &a2aAgent)

	// Create an incident (via webhook + ingest) for the lease
	whBody, _ := json.Marshal(models.CreateWebhookRequest{
		Name: "lease-wh", Source: "datadog", SoftwareID: sw.ID,
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/webhooks", whBody, token))
	require.Equal(t, http.StatusCreated, w.Code)
	var wh models.Webhook
	json.Unmarshal(w.Body.Bytes(), &wh)

	w = httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ingest/"+wh.EndpointToken,
		bytes.NewReader([]byte(`{"alert_id":"l1","alert_title":"Lease Test","alert_type":"error","priority":"high","tags":"service:pay"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	var ingestRes map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &ingestRes)
	incidentID, _ := uuid.Parse(ingestRes["incident_id"].(string))

	// Request lease
	leaseBody, _ := json.Marshal(models.RequestLeaseRequest{
		IncidentID:           incidentID,
		AgentID:              a2aAgent.ID,
		ResourceCredentialID: rc.ID,
		TTLSeconds:           300,
		Reason:               "Need DB access for evidence collection",
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", "/api/v1/credentials/leases/request", leaseBody, token))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var lease models.CredentialLease
	json.Unmarshal(w.Body.Bytes(), &lease)
	assert.Equal(t, "active", lease.Status)

	// List active leases
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/credentials/leases/active", nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var activeLeases []models.CredentialLease
	json.Unmarshal(w.Body.Bytes(), &activeLeases)
	assert.GreaterOrEqual(t, len(activeLeases), 1)

	// Revoke lease
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("POST", fmt.Sprintf("/api/v1/credentials/leases/%s/revoke", lease.ID), nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var revoked models.CredentialLease
	json.Unmarshal(w.Body.Bytes(), &revoked)
	assert.Equal(t, "revoked", revoked.Status)
}

// --- Pagination ---

func TestIntegration_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	r, _ := setupTestRouter(t, pool)
	orgID := createTestOrg(t, pool)
	_, token := createTestUser(t, pool, orgID)
	t.Cleanup(func() { cleanupTestData(t, pool, orgID) })

	// Create 25 software entries
	for i := 0; i < 25; i++ {
		body, _ := json.Marshal(models.CreateSoftwareRequest{
			Name: fmt.Sprintf("svc-%03d", i),
			Slug: fmt.Sprintf("svc-%03d", i),
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authRequest("POST", "/api/v1/software", body, token))
		require.Equal(t, http.StatusCreated, w.Code, "creating svc-%03d: %s", i, w.Body.String())
	}

	// Page 1 (default per_page=20)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/software?page=1", nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var page1 models.PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &page1)
	assert.Equal(t, 25, page1.Total)
	assert.Equal(t, 1, page1.Page)
	assert.Equal(t, 20, page1.PerPage)

	// Page 2
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/software?page=2", nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var page2 models.PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &page2)
	assert.Equal(t, 25, page2.Total)
	assert.Equal(t, 2, page2.Page)

	// per_page > 100 should be clamped
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/software?per_page=200", nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var clamped models.PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &clamped)
	assert.Equal(t, 100, clamped.PerPage)

	// page < 1 defaults to 1
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authRequest("GET", "/api/v1/software?page=0", nil, token))
	require.Equal(t, http.StatusOK, w.Code)
	var defaulted models.PaginatedResponse
	json.Unmarshal(w.Body.Bytes(), &defaulted)
	assert.Equal(t, 1, defaulted.Page)
}
