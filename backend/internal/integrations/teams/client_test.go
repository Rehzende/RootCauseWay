package teams

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withStubbedGraph points GraphBaseURL and MicrosoftLoginBaseURL at srv for
// the duration of the test, restoring the real values afterwards -- these
// are shared package vars (see their doc comment in client.go), so every
// test that touches them must clean up.
func withStubbedGraph(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origGraph, origLogin, origRetryUnit := GraphBaseURL, MicrosoftLoginBaseURL, onlineMeetingAttachRetryUnit
	GraphBaseURL = srv.URL
	MicrosoftLoginBaseURL = srv.URL
	onlineMeetingAttachRetryUnit = time.Millisecond
	t.Cleanup(func() {
		GraphBaseURL = origGraph
		MicrosoftLoginBaseURL = origLogin
		onlineMeetingAttachRetryUnit = origRetryUnit
	})
}

func tokenHandler(t *testing.T, wantGrantType string, resp oauthTokenResponse) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, wantGrantType, r.PostForm.Get("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}
}

// calendarMeetingHandler simulates the 3-request calendar-backed meeting
// creation flow CreateMeeting now drives: POST .../events (create the
// event + Teams meeting), GET .../onlineMeetings?$filter=... (resolve the
// onlineMeeting's own id from the event's join URL), PATCH
// .../onlineMeetings/{id} (enable auto-record/transcription). Records every
// non-token request path+method it sees into *seen, in order.
func calendarMeetingHandler(t *testing.T, meetingID, joinURL string, seen *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/calendar"):
			_ = json.NewEncoder(w).Encode(map[string]any{"allowedOnlineMeetingProviders": []string{"teamsForBusiness"}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "event-1",
				"onlineMeeting": map[string]string{"joinUrl": joinURL},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/onlineMeetings"):
			assert.Contains(t, r.URL.RawQuery, "%24filter", "must send an OData $filter query param")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]string{{"id": meetingID}},
			})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/onlineMeetings/"):
			w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}
}

func withTokenAndCalendarMeeting(grantType, accessToken, meetingID, joinURL string, seen *[]string, t *testing.T) http.HandlerFunc {
	meetingHandler := calendarMeetingHandler(t, meetingID, joinURL, seen)
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/oauth2/v2.0/token") {
			assert.Equal(t, grantType, r.PostFormValue("grant_type"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: accessToken, ExpiresIn: 3600})
			return
		}
		meetingHandler(w, r)
	}
}

// --- clientCredentialsTokenProvider / NewGraphTeamsClient (app-only, the
// unchanged legacy path) ---------------------------------------------------

func TestGraphTeamsClient_AppOnly_CreatesMeetingUnderOrganizerPath(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(withTokenAndCalendarMeeting(
		"client_credentials", "app-only-token", "meeting-1", "https://join", &seen, t,
	))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClient("tenant", "client", "secret", "organizer-42")
	id, joinURL, err := client.CreateMeeting(context.Background(), "Incident review")

	require.NoError(t, err)
	assert.Equal(t, "meeting-1", id)
	assert.Equal(t, "https://join", joinURL)
	require.Len(t, seen, 4, "expected check-calendar, create-event, resolve-by-joinurl, then enable-auto-record")
	assert.Equal(t, "GET /users/organizer-42/calendar", seen[0])
	assert.Equal(t, "POST /users/organizer-42/events", seen[1], "app-only auth must still target the fixed organizer path")
	assert.Contains(t, seen[2], "GET /users/organizer-42/onlineMeetings")
	assert.Equal(t, "PATCH /users/organizer-42/onlineMeetings/meeting-1", seen[3])
}

func TestGraphTeamsClient_AppOnly_CachesTokenAcrossCalls(t *testing.T) {
	tokenRequests := 0
	var seen []string
	meetingHandler := calendarMeetingHandler(t, "m", "https://join", &seen)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/oauth2/v2.0/token") {
			tokenRequests++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}
		meetingHandler(w, r)
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClient("tenant", "client", "secret", "organizer")
	_, _, err := client.CreateMeeting(context.Background(), "one")
	require.NoError(t, err)
	_, _, err = client.CreateMeeting(context.Background(), "two")
	require.NoError(t, err)

	assert.Equal(t, 1, tokenRequests, "a still-valid cached token must not be re-fetched")
}

// --- delegatedTokenProvider / NewGraphTeamsClientDelegated -----------------

func TestGraphTeamsClient_Delegated_CreatesMeetingUnderMePath(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(withTokenAndCalendarMeeting(
		"refresh_token", "delegated-token", "meeting-2", "https://join2", &seen, t,
	))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "old-refresh-token", nil)
	id, joinURL, err := client.CreateMeeting(context.Background(), "War room")

	require.NoError(t, err)
	assert.Equal(t, "meeting-2", id)
	assert.Equal(t, "https://join2", joinURL)
	require.Len(t, seen, 4)
	assert.Equal(t, "GET /me/calendar", seen[0])
	assert.Equal(t, "POST /me/events", seen[1], "delegated auth must create the meeting as the connected account, not an impersonated organizer")
	assert.Contains(t, seen[2], "GET /me/onlineMeetings")
	assert.Equal(t, "PATCH /me/onlineMeetings/meeting-2", seen[3])
}

func TestGraphTeamsClient_CreateMeeting_OnlineMeetingNeverAttaches_ErrorsAfterRetrying(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/oauth2/v2.0/token") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: "tok", ExpiresIn: 3600})
			return
		}
		// Event created, but no onlineMeeting ever comes back with it, on
		// the initial create or any re-fetch.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "event-1"})
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "refresh-token", nil)
	_, _, err := client.CreateMeeting(context.Background(), "subject")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no online meeting join URL")
}

// TestGraphTeamsClient_CreateMeeting_RetriesUntilOnlineMeetingAttaches
// guards a bug found live: Microsoft sometimes attaches the Teams meeting
// to a freshly created event asynchronously, so the immediate POST
// response's onlineMeeting object can come back empty even though the
// event itself was created successfully. CreateMeeting must re-fetch the
// event a few times before giving up, not fail on the very first empty
// response.
func TestGraphTeamsClient_CreateMeeting_RetriesUntilOnlineMeetingAttaches(t *testing.T) {
	var getCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/v2.0/token"):
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: "tok", ExpiresIn: 3600})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events"):
			// Event created, Teams meeting not attached yet.
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "event-1"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events/event-1"):
			getCount++
			if getCount < 2 {
				// Still not attached on the first re-fetch either.
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "event-1"})
				return
			}
			// Attached by the second re-fetch.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "event-1",
				"onlineMeeting": map[string]string{"joinUrl": "https://join"},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/onlineMeetings"):
			_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]string{{"id": "meeting-1"}}})
		case r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "refresh-token", nil)
	id, joinURL, err := client.CreateMeeting(context.Background(), "subject")

	require.NoError(t, err)
	assert.Equal(t, "meeting-1", id)
	assert.Equal(t, "https://join", joinURL)
	assert.Equal(t, 2, getCount, "expected exactly 2 re-fetches before the online meeting was attached")
}

func TestGraphTeamsClient_CreateMeeting_ResolveFails_Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/v2.0/token"):
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: "tok", ExpiresIn: 3600})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "event-1",
				"onlineMeeting": map[string]string{"joinUrl": "https://join"},
			})
		case r.Method == http.MethodGet:
			// Graph found nothing for this join URL.
			_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]string{}})
		}
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "refresh-token", nil)
	_, _, err := client.CreateMeeting(context.Background(), "subject")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve online meeting")
}

func TestGraphTeamsClient_CreateMeeting_AutoRecordPatchFails_StillSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/v2.0/token"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: "tok", ExpiresIn: 3600})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "event-1",
				"onlineMeeting": map[string]string{"joinUrl": "https://join"},
			})
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]string{{"id": "meeting-1"}}})
		case r.Method == http.MethodPatch:
			// Enabling auto-record/transcription fails -- must not fail
			// the whole meeting creation over this.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"insufficient permissions"}`))
		}
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "refresh-token", nil)
	id, joinURL, err := client.CreateMeeting(context.Background(), "subject")

	require.NoError(t, err)
	assert.Equal(t, "meeting-1", id)
	assert.Equal(t, "https://join", joinURL)
}

func TestGraphTeamsClient_Delegated_PersistsRotatedRefreshToken(t *testing.T) {
	var seen []string
	meetingHandler := calendarMeetingHandler(t, "m", "https://join", &seen)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/oauth2/v2.0/token") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{
				AccessToken: "tok", RefreshToken: "rotated-refresh-token", ExpiresIn: 3600,
			})
			return
		}
		meetingHandler(w, r)
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	var persisted string
	persistCalls := 0
	persist := func(ctx context.Context, newToken string) error {
		persistCalls++
		persisted = newToken
		return nil
	}

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "old-refresh-token", persist)
	_, _, err := client.CreateMeeting(context.Background(), "subject")

	require.NoError(t, err)
	assert.Equal(t, 1, persistCalls)
	assert.Equal(t, "rotated-refresh-token", persisted)
}

func TestGraphTeamsClient_Delegated_DoesNotPersistWhenTokenUnchanged(t *testing.T) {
	var seen []string
	meetingHandler := calendarMeetingHandler(t, "m", "https://join", &seen)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/oauth2/v2.0/token") {
			w.Header().Set("Content-Type", "application/json")
			// Microsoft returns the SAME refresh_token back (allowed, not
			// guaranteed to rotate every single time).
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{
				AccessToken: "tok", RefreshToken: "same-refresh-token", ExpiresIn: 3600,
			})
			return
		}
		meetingHandler(w, r)
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	persistCalls := 0
	persist := func(ctx context.Context, newToken string) error {
		persistCalls++
		return nil
	}

	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "same-refresh-token", persist)
	_, _, err := client.CreateMeeting(context.Background(), "subject")

	require.NoError(t, err)
	assert.Equal(t, 0, persistCalls, "must not persist when the refresh token didn't actually change")
}

func TestGraphTeamsClient_Delegated_NilPersistCallback_DoesNotCrashOnRotation(t *testing.T) {
	srv := httptest.NewServer(tokenHandler(t, "refresh_token", oauthTokenResponse{
		AccessToken: "tok", RefreshToken: "rotated-refresh-token", ExpiresIn: 3600,
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	// persistRefreshToken is nil -- must not panic even though Microsoft
	// sends back a genuinely rotated token here.
	client := NewGraphTeamsClientDelegated("tenant", "client", "secret", "refresh-token", nil)
	token, err := client.tokens.getToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok", token)
}

// --- Standalone oauth.go functions ------------------------------------

func TestBuildAuthorizeURL(t *testing.T) {
	got := BuildAuthorizeURL("my-tenant", "my-client", "https://api.example.com/callback", "state-123")

	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "/my-tenant/oauth2/v2.0/authorize", u.Path)
	q := u.Query()
	assert.Equal(t, "my-client", q.Get("client_id"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "https://api.example.com/callback", q.Get("redirect_uri"))
	assert.Equal(t, "state-123", q.Get("state"))
	assert.Contains(t, q.Get("scope"), "offline_access")
	assert.Contains(t, q.Get("scope"), "OnlineMeetings.ReadWrite")
	assert.Contains(t, q.Get("scope"), "Calendars.ReadWrite")
	// Found missing live: without these, EndWarRoom "succeeds" (reaches
	// status "summarized") but GetTranscript/GetAttendanceReport both 403,
	// silently producing an empty "No transcript available" summary.
	assert.Contains(t, q.Get("scope"), "OnlineMeetingTranscript.Read.All")
	assert.Contains(t, q.Get("scope"), "OnlineMeetingArtifact.Read.All")
	assert.Equal(t, "consent", q.Get("prompt"), "must force a fresh consent screen every time -- see doc comment for why")
}

func TestExchangeCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.PostForm.Get("grant_type"))
		assert.Equal(t, "auth-code-abc", r.PostForm.Get("code"))
		assert.Equal(t, "https://api.example.com/callback", r.PostForm.Get("redirect_uri"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken: "access-1", RefreshToken: "refresh-1", ExpiresIn: 3600,
		})
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	access, refresh, err := ExchangeCode(context.Background(), "tenant", "client", "secret", "auth-code-abc", "https://api.example.com/callback")

	require.NoError(t, err)
	assert.Equal(t, "access-1", access)
	assert.Equal(t, "refresh-1", refresh)
}

func TestExchangeCode_NoRefreshTokenInResponse_Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{AccessToken: "access-1", ExpiresIn: 3600})
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	_, _, err := ExchangeCode(context.Background(), "tenant", "client", "secret", "code", "https://redirect")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no refresh_token")
}

func TestExchangeCode_TokenEndpointError_Propagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	_, _, err := ExchangeCode(context.Background(), "tenant", "client", "secret", "bad-code", "https://redirect")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestFetchMe_PrefersUserPrincipalNameOverMail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer access-1", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(graphMeResponse{
			UserPrincipalName: "rootcauseway-bot@customer.onmicrosoft.com",
			Mail:              "rootcauseway-bot@customer.com",
		})
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	email, err := FetchMe(context.Background(), "access-1")
	require.NoError(t, err)
	assert.Equal(t, "rootcauseway-bot@customer.onmicrosoft.com", email)
}

func TestFetchMe_FallsBackToMailWhenNoUserPrincipalName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(graphMeResponse{Mail: "rootcauseway-bot@customer.com"})
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	email, err := FetchMe(context.Background(), "access-1")
	require.NoError(t, err)
	assert.Equal(t, "rootcauseway-bot@customer.com", email)
}

// --- accessTokenScopes ------------------------------------------------

func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payloadBytes, err := json.Marshal(claims)
	require.NoError(t, err)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".fake-signature"
}

func TestAccessTokenScopes_DecodesScpClaim(t *testing.T) {
	token := fakeJWT(t, map[string]any{"scp": "Calendars.ReadWrite OnlineMeetings.ReadWrite User.Read"})

	scopes, err := accessTokenScopes(token)

	require.NoError(t, err)
	assert.Equal(t, "Calendars.ReadWrite OnlineMeetings.ReadWrite User.Read", scopes)
}

func TestAccessTokenScopes_NotAJWT_Errors(t *testing.T) {
	_, err := accessTokenScopes("not-a-jwt-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a JWT")
}
