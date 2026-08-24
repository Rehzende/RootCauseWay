// Package teams provides a provider-agnostic interface for creating and
// retrieving data about video-conferencing "war room" meetings, plus a
// real Microsoft Teams (Graph API) implementation and two test/dev
// fallbacks (Mock and Noop).
//
// Selection between implementations is entirely config-driven (env vars),
// so swapping in real Azure AD tenant credentials later requires zero code
// changes -- see NewClientFromEnv.
package teams

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Attendee is a single participant captured in a meeting's attendance
// report.
type Attendee struct {
	Name      string     `json:"name,omitempty"`
	Email     string     `json:"email,omitempty"`
	JoinTime  *time.Time `json:"join_time,omitempty"`
	LeaveTime *time.Time `json:"leave_time,omitempty"`
}

// TeamsClient is implemented by every meeting-provider backend (real Graph
// API, mock, noop). Services depend on this interface only.
type TeamsClient interface {
	// CreateMeeting creates a new online meeting and returns its
	// provider-side ID and a join URL. attendees is best-effort: a nil or
	// empty slice creates a meeting with no invited attendees (organizer
	// only), same as before this parameter existed.
	CreateMeeting(ctx context.Context, subject string, attendees []Attendee) (externalID, joinURL string, err error)
	// GetTranscript returns the raw transcript text for an ended meeting.
	GetTranscript(ctx context.Context, externalID string) (string, error)
	// GetAttendanceReport returns the list of attendees for an ended
	// meeting.
	GetAttendanceReport(ctx context.Context, externalID string) ([]Attendee, error)
}

// NewClientFromEnv selects a TeamsClient implementation based on
// environment configuration. This is the legacy, process-wide, single-tenant
// path (superseded per-org by services.NewTeamsClientResolver, which builds
// a delegated-OAuth GraphTeamsClient from each org's stored settings) --
// still useful for local dev without touching Postgres.
//
//   - TEAMS_TENANT_ID, TEAMS_CLIENT_ID, TEAMS_CLIENT_SECRET, and
//     TEAMS_ORGANIZER_USER_ID all set -> GraphTeamsClient (real Microsoft
//     Graph API, app-only client-credentials auth). Note: app-only auth
//     additionally requires a tenant admin to have granted an Application
//     Access Policy via PowerShell for CreateMeeting to work against an
//     arbitrary organizer -- see NewGraphTeamsClientDelegated for the
//     alternative that doesn't need that.
//   - Those unset AND WARROOM_MOCK_MODE=true -> MockTeamsClient (canned
//     responses, safe for local dev / demos without an Azure tenant).
//   - Otherwise -> NoopTeamsClient, which fails clearly on every call so
//     the app still boots without Azure credentials configured.
func NewClientFromEnv() TeamsClient {
	tenantID := os.Getenv("TEAMS_TENANT_ID")
	clientID := os.Getenv("TEAMS_CLIENT_ID")
	clientSecret := os.Getenv("TEAMS_CLIENT_SECRET")
	organizerUserID := os.Getenv("TEAMS_ORGANIZER_USER_ID")

	if tenantID != "" && clientID != "" && clientSecret != "" && organizerUserID != "" {
		return NewGraphTeamsClient(tenantID, clientID, clientSecret, organizerUserID)
	}

	if strings.EqualFold(os.Getenv("WARROOM_MOCK_MODE"), "true") {
		return NewMockTeamsClient()
	}

	return NewNoopTeamsClient()
}

// --- Noop implementation -----------------------------------------------

// NoopTeamsClient is the default when no Teams credentials are configured
// and mock mode isn't explicitly requested. It returns a clear,
// actionable error on every call instead of silently no-op'ing, so
// misconfiguration is obvious in logs/responses rather than producing
// confusing empty state.
type NoopTeamsClient struct{}

func NewNoopTeamsClient() *NoopTeamsClient { return &NoopTeamsClient{} }

var errNotConfigured = fmt.Errorf(
	"Teams integration not configured: set TEAMS_TENANT_ID, TEAMS_CLIENT_ID, " +
		"TEAMS_CLIENT_SECRET and TEAMS_ORGANIZER_USER_ID, or set WARROOM_MOCK_MODE=true for local dev",
)

func (c *NoopTeamsClient) CreateMeeting(ctx context.Context, subject string, attendees []Attendee) (string, string, error) {
	return "", "", errNotConfigured
}

func (c *NoopTeamsClient) GetTranscript(ctx context.Context, externalID string) (string, error) {
	return "", errNotConfigured
}

func (c *NoopTeamsClient) GetAttendanceReport(ctx context.Context, externalID string) ([]Attendee, error) {
	return nil, errNotConfigured
}

// --- Mock implementation -------------------------------------------------

// MockTeamsClient returns canned data. It's used by Go/Python tests and
// is also selectable as a dev-mode fallback via WARROOM_MOCK_MODE=true,
// so the full war-room flow (create -> end -> summarize) can be exercised
// end-to-end without an Azure AD tenant.
type MockTeamsClient struct {
	mu      sync.Mutex
	counter int
}

func NewMockTeamsClient() *MockTeamsClient { return &MockTeamsClient{} }

func (c *MockTeamsClient) CreateMeeting(ctx context.Context, subject string, attendees []Attendee) (string, string, error) {
	c.mu.Lock()
	c.counter++
	n := c.counter
	c.mu.Unlock()
	externalID := fmt.Sprintf("mock-meeting-%d", n)
	joinURL := fmt.Sprintf("https://teams.microsoft.com/l/meetup-join/mock-%d", n)
	return externalID, joinURL, nil
}

const mockTranscript = `[00:00:01] Alice: Kicking off the war room for the checkout-service outage.
[00:00:15] Bob: I see 502s spiking on the payments gateway since 14:02 UTC.
[00:02:30] Alice: Let's roll back the last deploy while we investigate.
[00:05:10] Carol: Rollback is out. Error rate is dropping.
[00:08:00] Bob: Confirmed, checkout is healthy again. I'll open a ticket to add a canary check before next deploy.
[00:09:45] Alice: Great, I'll draft the postmortem. Bob, can you own the canary check action item?
[00:10:00] Bob: Yes, I'll have it by Friday.`

func (c *MockTeamsClient) GetTranscript(ctx context.Context, externalID string) (string, error) {
	return mockTranscript, nil
}

func (c *MockTeamsClient) GetAttendanceReport(ctx context.Context, externalID string) ([]Attendee, error) {
	now := time.Now().UTC()
	join := now.Add(-10 * time.Minute)
	leave := now
	return []Attendee{
		{Name: "Alice Smith", Email: "alice@example.com", JoinTime: &join, LeaveTime: &leave},
		{Name: "Bob Jones", Email: "bob@example.com", JoinTime: &join, LeaveTime: &leave},
		{Name: "Carol Lee", Email: "carol@example.com", JoinTime: &join, LeaveTime: &leave},
	}, nil
}

// --- Real Microsoft Graph implementation ---------------------------------

// GraphBaseURL and MicrosoftLoginBaseURL are exported package vars (not
// consts) so tests -- including tests in OTHER packages, like
// services.TeamsOAuthService's, which drives this package's OAuth
// functions end to end -- can point them at an httptest.Server instead of
// the real Microsoft endpoints. Same dependency-injection style as
// backend/internal/services/azure_keyvault_provider.go's newAzureSecretGetter
// package var. Production code never reassigns these; tests that do MUST
// restore the original value afterwards (e.g. via t.Cleanup), since these
// are shared, mutable package state.
var (
	GraphBaseURL          = "https://graph.microsoft.com/v1.0"
	MicrosoftLoginBaseURL = "https://login.microsoftonline.com"
)

// tokenProvider abstracts how a GraphTeamsClient gets a bearer token, so the
// same client/request logic (doJSON, CreateMeeting, GetTranscript,
// GetAttendanceReport) works whether the token comes from app-only
// client_credentials auth or a delegated refresh_token grant.
type tokenProvider interface {
	getToken(ctx context.Context) (string, error)
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// postTokenForm POSTs a token-endpoint request shared by every grant type
// (client_credentials, authorization_code, refresh_token) -- they differ
// only in the form fields, not the transport/parsing.
func postTokenForm(ctx context.Context, httpClient *http.Client, tenantID string, form url.Values) (oauthTokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", MicrosoftLoginBaseURL, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return oauthTokenResponse{}, fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return oauthTokenResponse{}, fmt.Errorf("token request failed: %d %s", resp.StatusCode, string(body))
	}

	var tr oauthTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return oauthTokenResponse{}, fmt.Errorf("decode token response: %w", err)
	}
	return tr, nil
}

// clientCredentialsTokenProvider is today's app-only auth (unchanged
// behavior from before this file's tokenProvider refactor): a fixed service
// identity, no user ever consents, meetings are created "as" whichever user
// GraphTeamsClient's basePath names -- which is why it requires a tenant
// admin to grant a Microsoft Application Access Policy.
type clientCredentialsTokenProvider struct {
	tenantID, clientID, clientSecret string
	httpClient                       *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func (p *clientCredentialsTokenProvider) getToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("scope", "https://graph.microsoft.com/.default")

	tr, err := postTokenForm(ctx, p.httpClient, p.tenantID, form)
	if err != nil {
		return "", err
	}
	p.accessToken = tr.AccessToken
	// Refresh a minute early to avoid races with expiry.
	p.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)
	return p.accessToken, nil
}

// delegatedTokenProvider authenticates as a connected service/bot account
// via a stored OAuth refresh token (see package teams's oauth.go for the
// authorization-code flow that produces the initial one). Microsoft rotates
// the refresh token on every use, so a successful refresh calls
// persistRefreshToken with the new value -- losing that would mean the next
// refresh has to fall back to an older token, which Microsoft still honors
// for a grace period but shouldn't be relied on indefinitely.
type delegatedTokenProvider struct {
	tenantID, clientID, clientSecret string
	httpClient                       *http.Client
	persistRefreshToken              func(ctx context.Context, newRefreshToken string) error

	mu           sync.Mutex
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
}

func (p *delegatedTokenProvider) getToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("refresh_token", p.refreshToken)
	// Must match (or be a subset of) teamsOAuthScopes (oauth.go) -- a
	// refresh request can only ever narrow the already-consented scopes,
	// never exceed them, so requesting less here than what CreateMeeting
	// actually needs (Calendars.ReadWrite, for the calendar-backed meeting
	// creation) would silently produce an access token missing it.
	form.Set("scope", teamsOAuthScopes)

	tr, err := postTokenForm(ctx, p.httpClient, p.tenantID, form)
	if err != nil {
		return "", fmt.Errorf("refresh delegated token: %w", err)
	}
	p.accessToken = tr.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)

	// Diagnostic: log what scopes actually ended up on this access token
	// (the "scp" claim), rather than assuming the consent/refresh scope
	// request took effect. Cuts through guessing about consent screens
	// entirely -- this is the ground truth of what Graph will actually
	// authorize this specific call with.
	if scopes, err := accessTokenScopes(p.accessToken); err != nil {
		slog.Warn("could not decode delegated access token to inspect its scopes", "error", err)
	} else {
		slog.Info("delegated Teams access token scopes", "scopes", scopes)
	}

	if tr.RefreshToken != "" && tr.RefreshToken != p.refreshToken {
		p.refreshToken = tr.RefreshToken
		if p.persistRefreshToken != nil {
			if err := p.persistRefreshToken(ctx, tr.RefreshToken); err != nil {
				// Don't fail the in-flight Graph call over a persistence
				// hiccup -- the access token we just obtained is still
				// valid for it. Just log loudly: if this keeps failing,
				// the org's Teams integration will eventually break once
				// Microsoft stops accepting the stale refresh token.
				slog.Error("failed to persist rotated Teams refresh token", "error", err)
			}
		}
	}
	return p.accessToken, nil
}

// GraphTeamsClient implements TeamsClient against the real Microsoft Graph
// API. Auth is delegated to tokenProvider (app-only client_credentials via
// NewGraphTeamsClient, or delegated refresh_token via
// NewGraphTeamsClientDelegated) -- basePath then decides whether requests
// target an impersonated organizer (/users/{id}) or the token's own
// identity (/me).
// accessTokenScopes decodes (without verifying -- this is a read-only
// diagnostic, not a security check, we already trust a token we ourselves
// just obtained from Microsoft) the "scp" claim out of a JWT access
// token's payload, to see exactly what Graph will authorize this token
// for. Access tokens for personal/guest scenarios are sometimes opaque
// (not a JWT) -- returns an error in that case rather than panicking.
func accessTokenScopes(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("access token is not a JWT (got %d dot-separated parts)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims struct {
		Scope string `json:"scp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse JWT claims: %w", err)
	}
	return claims.Scope, nil
}

type GraphTeamsClient struct {
	tokens     tokenProvider
	basePath   string
	httpClient *http.Client
}

// NewGraphTeamsClient builds a GraphTeamsClient using app-only auth
// (OAuth2 client-credentials flow), creating meetings under a fixed
// "organizer" service account since app-only auth cannot create a meeting
// "as" an arbitrary user without delegated permissions. This additionally
// requires a tenant admin to have granted a Microsoft Application Access
// Policy via PowerShell for organizerUserID -- see
// NewGraphTeamsClientDelegated for the delegated-OAuth alternative that
// doesn't need that.
func NewGraphTeamsClient(tenantID, clientID, clientSecret, organizerUserID string) *GraphTeamsClient {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &GraphTeamsClient{
		tokens: &clientCredentialsTokenProvider{
			tenantID:     tenantID,
			clientID:     clientID,
			clientSecret: clientSecret,
			httpClient:   httpClient,
		},
		basePath:   "/users/" + organizerUserID,
		httpClient: httpClient,
	}
}

// NewGraphTeamsClientDelegated builds a GraphTeamsClient that authenticates
// via a delegated OAuth refresh token instead of app-only
// client_credentials -- see package teams's oauth.go for the
// authorization-code flow that produces the initial refresh token. Meetings
// are created as the connected service/bot account itself (/me/...) rather
// than an impersonated organizer, which is what lets this skip the
// Application Access Policy requirement entirely: the account consented to
// this app once, in a browser, like any other delegated Graph integration.
//
// persistRefreshToken is called whenever Microsoft rotates the refresh
// token (see delegatedTokenProvider) so the caller can keep its stored copy
// current.
func NewGraphTeamsClientDelegated(
	tenantID, clientID, clientSecret, refreshToken string,
	persistRefreshToken func(ctx context.Context, newRefreshToken string) error,
) *GraphTeamsClient {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &GraphTeamsClient{
		tokens: &delegatedTokenProvider{
			tenantID:            tenantID,
			clientID:            clientID,
			clientSecret:        clientSecret,
			refreshToken:        refreshToken,
			persistRefreshToken: persistRefreshToken,
			httpClient:          httpClient,
		},
		basePath:   "/me",
		httpClient: httpClient,
	}
}

func (c *GraphTeamsClient) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	token, err := c.tokens.getToken(ctx)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, GraphBaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build graph request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("graph request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("graph request %s %s failed: %d %s", method, path, resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode graph response for %s %s: %w", method, path, err)
		}
	}
	return nil
}

type eventDateTimeTZ struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type createEventRequest struct {
	Subject               string          `json:"subject"`
	Start                 eventDateTimeTZ `json:"start"`
	End                   eventDateTimeTZ `json:"end"`
	IsOnlineMeeting       bool            `json:"isOnlineMeeting"`
	OnlineMeetingProvider string          `json:"onlineMeetingProvider"`
	// Attendees is omitted entirely (not sent as []) when there are none --
	// Graph accepts a missing field the same as an empty array here, and
	// omitting keeps the request identical to before this field existed for
	// the common war-room-with-no-known-stakeholders case.
	Attendees []graphAttendee `json:"attendees,omitempty"`
}

// graphAttendee is the Calendar Event resource's attendee shape (Graph API
// "attendee" resource type) -- distinct from Attendee above, which is this
// package's own shape for an *ended* meeting's attendance report.
type graphAttendee struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
	Type         string            `json:"type"` // "required" | "optional" | "resource"
}

type graphEmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type createEventResponse struct {
	ID string `json:"id"`
	// IsOnlineMeeting/OnlineMeetingProvider are echoed back by Graph --
	// captured (not just OnlineMeeting.JoinURL) so a failure to attach can
	// be diagnosed: Graph silently accepting isOnlineMeeting:true but
	// echoing it back as false would mean the request itself is being
	// rejected/ignored, a different failure mode than "accepted but the
	// Teams meeting attachment genuinely never happens".
	IsOnlineMeeting       bool   `json:"isOnlineMeeting"`
	OnlineMeetingProvider string `json:"onlineMeetingProvider"`
	OnlineMeetingURL      string `json:"onlineMeetingUrl"`
	OnlineMeeting         struct {
		JoinURL string `json:"joinUrl"`
	} `json:"onlineMeeting"`
}

type onlineMeetingLookupResponse struct {
	Value []struct {
		ID string `json:"id"`
	} `json:"value"`
}

type updateOnlineMeetingRequest struct {
	RecordAutomatically bool `json:"recordAutomatically"`
	AllowTranscription  bool `json:"allowTranscription"`
}

// onlineMeetingAttachRetryUnit paces CreateMeeting's retries while waiting
// for Microsoft to asynchronously attach a Teams meeting to a freshly
// created event (1x, 2x, 3x this unit). A package var, not a const, so
// tests can shrink it to avoid real multi-second sleeps -- same pattern as
// GraphBaseURL/MicrosoftLoginBaseURL above.
var onlineMeetingAttachRetryUnit = time.Second

// warRoomCalendarBlockDuration is a calendar placeholder only -- war rooms
// don't have a known end time upfront, and ending the calendar block does
// NOT cut off the live Teams call itself (people can stay in it as long as
// they want; EndWarRoom is what actually signals "this is over" on RootCauseway's
// side). Long enough to comfortably cover a real incident war room without
// looking like an all-day block on the connected account's calendar.
const warRoomCalendarBlockDuration = 4 * time.Hour

// CreateMeeting creates a Teams online meeting via a calendar-backed event
// (POST {basePath}/events with isOnlineMeeting:true) rather than the
// simpler standalone POST {basePath}/onlineMeetings endpoint -- Microsoft's
// own docs say the calendar-backed form is required "to be able to
// retrieve meeting transcripts at a later stage" (confirmed live against
// https://learn.microsoft.com/en-us/graph/api/application-post-onlinemeetings;
// the standalone endpoint doesn't guarantee it). See teamsOAuthScopes'
// doc comment in oauth.go for the Calendars.ReadWrite scope this requires.
//
// The Teams meeting can take a moment to attach to a freshly created
// event (confirmed live -- the initial response sometimes has an empty
// onlineMeeting object even with isOnlineMeeting:true accepted), so this
// retries a few times before giving up.
//
// After that, this resolves the event's own onlineMeeting resource (the
// event response only gives its join URL, not the onlineMeeting's id --
// Graph doesn't return that directly) and enables automatic recording +
// transcription on it, best-effort: if that PATCH fails, the meeting is
// still created and still fully usable, just without auto-record
// (whoever joins can still start transcription manually from the Teams
// client, same as before this change).
func (c *GraphTeamsClient) CreateMeeting(ctx context.Context, subject string, attendees []Attendee) (string, string, error) {
	// Diagnostic, not fatal if it fails: the calendar itself declares
	// which online meeting providers it'll actually honor
	// (allowedOnlineMeetingProviders) -- if "teamsForBusiness" isn't in
	// that list, isOnlineMeeting:true on event creation gets silently
	// ignored regardless of the caller's Graph permissions, which is
	// indistinguishable from a permissions problem without this check
	// (confirmed against real-world reports of this exact symptom).
	var cal struct {
		AllowedOnlineMeetingProviders []string `json:"allowedOnlineMeetingProviders"`
	}
	if err := c.doJSON(ctx, http.MethodGet, c.basePath+"/calendar", nil, &cal); err != nil {
		slog.Warn("could not read calendar's allowedOnlineMeetingProviders", "error", err)
	} else {
		slog.Info("calendar allowed online meeting providers", "providers", cal.AllowedOnlineMeetingProviders)
	}

	start := time.Now().UTC()
	end := start.Add(warRoomCalendarBlockDuration)

	var graphAttendees []graphAttendee
	for _, a := range attendees {
		if a.Email == "" {
			continue // Graph rejects an attendee with no address at all
		}
		graphAttendees = append(graphAttendees, graphAttendee{
			EmailAddress: graphEmailAddress{Address: a.Email, Name: a.Name},
			Type:         "required",
		})
	}

	eventReq := createEventRequest{
		Subject:               subject,
		Start:                 eventDateTimeTZ{DateTime: start.Format("2006-01-02T15:04:05.0000000"), TimeZone: "UTC"},
		End:                   eventDateTimeTZ{DateTime: end.Format("2006-01-02T15:04:05.0000000"), TimeZone: "UTC"},
		IsOnlineMeeting:       true,
		OnlineMeetingProvider: "teamsForBusiness",
		Attendees:             graphAttendees,
	}
	var eventResp createEventResponse
	if err := c.doJSON(ctx, http.MethodPost, c.basePath+"/events", eventReq, &eventResp); err != nil {
		return "", "", fmt.Errorf("create calendar event: %w", err)
	}

	// Confirmed live: the immediate POST response sometimes comes back
	// with isOnlineMeeting/onlineMeetingProvider echoed but an empty
	// onlineMeeting object -- Microsoft attaches the actual Teams meeting
	// to the event asynchronously in some tenants, so it isn't always
	// ready by the time the create call returns. Re-fetch the event a few
	// times, a beat apart, before giving up.
	joinURL := eventResp.OnlineMeeting.JoinURL
	latest := eventResp
	for attempt := 0; joinURL == "" && attempt < 3; attempt++ {
		time.Sleep(time.Duration(attempt+1) * onlineMeetingAttachRetryUnit)
		var refetched createEventResponse
		if err := c.doJSON(ctx, http.MethodGet, c.basePath+"/events/"+eventResp.ID, nil, &refetched); err != nil {
			return "", "", fmt.Errorf("re-fetch calendar event while waiting for online meeting: %w", err)
		}
		latest = refetched
		joinURL = refetched.OnlineMeeting.JoinURL
	}
	if joinURL == "" {
		// Diagnostic detail for why this is happening, not just that it
		// is: if isOnlineMeeting/onlineMeetingProvider come back
		// different from what was sent, Graph is rejecting/ignoring the
		// request server-side rather than genuinely racing on
		// attachment.
		slog.Warn("calendar event never got a Teams online meeting attached",
			"event_id", latest.ID,
			"echoed_is_online_meeting", latest.IsOnlineMeeting,
			"echoed_online_meeting_provider", latest.OnlineMeetingProvider,
			"legacy_online_meeting_url", latest.OnlineMeetingURL,
		)
		return "", "", fmt.Errorf(
			"create calendar event: no online meeting join URL after retrying -- the connected " +
				"account may not be able to create Teams meetings (check its Teams license and " +
				"meeting policy in the Microsoft 365 admin center)",
		)
	}
	eventResp.OnlineMeeting.JoinURL = joinURL

	meetingID, err := c.resolveOnlineMeetingID(ctx, eventResp.OnlineMeeting.JoinURL)
	if err != nil {
		return "", "", fmt.Errorf("resolve online meeting from calendar event: %w", err)
	}

	patchPath := fmt.Sprintf("%s/onlineMeetings/%s", c.basePath, meetingID)
	patchReq := updateOnlineMeetingRequest{RecordAutomatically: true, AllowTranscription: true}
	if err := c.doJSON(ctx, http.MethodPatch, patchPath, patchReq, nil); err != nil {
		slog.Warn("failed to enable automatic recording/transcription for war room meeting", "meeting_id", meetingID, "error", err)
	}

	return meetingID, eventResp.OnlineMeeting.JoinURL, nil
}

// resolveOnlineMeetingID looks up the onlineMeeting resource created
// alongside a calendar event by its join URL -- creating the event
// (CreateMeeting above) only returns that URL, not the onlineMeeting's own
// id, which GetTranscript/GetAttendanceReport need. This $filter pattern is
// Microsoft's documented way to make that connection for calendar-backed
// meetings.
func (c *GraphTeamsClient) resolveOnlineMeetingID(ctx context.Context, joinURL string) (string, error) {
	q := url.Values{}
	q.Set("$filter", fmt.Sprintf("JoinWebUrl eq '%s'", joinURL))
	listPath := fmt.Sprintf("%s/onlineMeetings?%s", c.basePath, q.Encode())

	var list onlineMeetingLookupResponse
	if err := c.doJSON(ctx, http.MethodGet, listPath, nil, &list); err != nil {
		return "", err
	}
	if len(list.Value) == 0 {
		return "", fmt.Errorf("no onlineMeeting found for join URL")
	}
	return list.Value[0].ID, nil
}

type transcriptListResponse struct {
	Value []struct {
		ID string `json:"id"`
	} `json:"value"`
}

// GetTranscript fetches the most recent transcript for a meeting via
// GET {basePath}/onlineMeetings/{id}/transcripts, then downloads its
// content.
func (c *GraphTeamsClient) GetTranscript(ctx context.Context, externalID string) (string, error) {
	listPath := fmt.Sprintf("%s/onlineMeetings/%s/transcripts", c.basePath, externalID)
	var list transcriptListResponse
	if err := c.doJSON(ctx, http.MethodGet, listPath, nil, &list); err != nil {
		return "", fmt.Errorf("list transcripts: %w", err)
	}
	if len(list.Value) == 0 {
		return "", fmt.Errorf("no transcripts available for meeting %s", externalID)
	}
	// Transcripts are returned newest-first by Graph; take the latest.
	transcriptID := list.Value[0].ID

	token, err := c.tokens.getToken(ctx)
	if err != nil {
		return "", err
	}
	contentPath := fmt.Sprintf("%s%s/onlineMeetings/%s/transcripts/%s/content",
		GraphBaseURL, c.basePath, externalID, transcriptID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, contentPath, nil)
	if err != nil {
		return "", fmt.Errorf("build transcript content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// text/vtt is the default caption format Graph returns; it's plain
	// text and fine to store/summarize as-is.
	req.Header.Set("Accept", "text/vtt")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch transcript content: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch transcript content failed: %d %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}

type attendanceReportListResponse struct {
	Value []struct {
		ID string `json:"id"`
	} `json:"value"`
}

type attendanceInterval struct {
	JoinDateTime  string `json:"joinDateTime"`
	LeaveDateTime string `json:"leaveDateTime"`
}

type attendanceRecord struct {
	Identity struct {
		DisplayName string `json:"displayName"`
	} `json:"identity"`
	EmailAddress        string               `json:"emailAddress"`
	AttendanceIntervals []attendanceInterval `json:"attendanceIntervals"`
}

type attendanceReportResponse struct {
	ID                string             `json:"id"`
	AttendanceRecords []attendanceRecord `json:"attendanceRecords"`
}

// GetAttendanceReport fetches the most recent attendance report for a
// meeting via GET {basePath}/onlineMeetings/{id}/attendanceReports,
// expanded with attendanceRecords.
func (c *GraphTeamsClient) GetAttendanceReport(ctx context.Context, externalID string) ([]Attendee, error) {
	listPath := fmt.Sprintf("%s/onlineMeetings/%s/attendanceReports", c.basePath, externalID)
	var list attendanceReportListResponse
	if err := c.doJSON(ctx, http.MethodGet, listPath, nil, &list); err != nil {
		return nil, fmt.Errorf("list attendance reports: %w", err)
	}
	if len(list.Value) == 0 {
		return nil, fmt.Errorf("no attendance reports available for meeting %s", externalID)
	}
	reportID := list.Value[0].ID

	detailPath := fmt.Sprintf("%s/onlineMeetings/%s/attendanceReports/%s?$expand=attendanceRecords",
		c.basePath, externalID, reportID)
	var report attendanceReportResponse
	if err := c.doJSON(ctx, http.MethodGet, detailPath, nil, &report); err != nil {
		return nil, fmt.Errorf("get attendance report: %w", err)
	}

	attendees := make([]Attendee, 0, len(report.AttendanceRecords))
	for _, rec := range report.AttendanceRecords {
		a := Attendee{Name: rec.Identity.DisplayName, Email: rec.EmailAddress}
		if len(rec.AttendanceIntervals) > 0 {
			if t, err := time.Parse(time.RFC3339, rec.AttendanceIntervals[0].JoinDateTime); err == nil {
				a.JoinTime = &t
			}
			last := rec.AttendanceIntervals[len(rec.AttendanceIntervals)-1]
			if t, err := time.Parse(time.RFC3339, last.LeaveDateTime); err == nil {
				a.LeaveTime = &t
			}
		}
		attendees = append(attendees, a)
	}
	return attendees, nil
}
