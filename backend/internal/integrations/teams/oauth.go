package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// oauthHTTPClient backs the standalone functions below (BuildAuthorizeURL
// has no HTTP call, but ExchangeCode/FetchMe do) -- separate from
// GraphTeamsClient's own httpClient since these run before any
// GraphTeamsClient exists yet (they're what PRODUCE the refresh token a
// GraphTeamsClient is later constructed with).
var oauthHTTPClient = &http.Client{Timeout: 30 * time.Second}

// teamsOAuthScopes is what the delegated Teams integration actually needs:
// offline_access (to get a refresh token back at all), openid+profile+
// User.Read (so FetchMe's GET /me works, for displaying which account got
// connected), OnlineMeetings.ReadWrite (to create/inspect meetings as that
// account), and Calendars.ReadWrite -- meetings are created via the
// calendar-backed POST /events (see GraphTeamsClient.CreateMeeting), not
// the standalone POST /onlineMeetings, because Microsoft's own docs say
// that's required "to be able to retrieve meeting transcripts at a later
// stage" (https://learn.microsoft.com/en-us/graph/api/application-post-onlinemeetings,
// confirmed live -- the standalone endpoint doesn't guarantee it).
// OnlineMeetingTranscript.Read.All and OnlineMeetingArtifact.Read.All are
// what GetTranscript/GetAttendanceReport actually need -- found missing
// live: EndWarRoom completed "successfully" (meeting reached status
// "summarized") but both Graph calls 403'd ("Missing scope permissions...
// API requires one of 'OnlineMeetingTranscript.Read.All'" /
// "Insufficient permissions"), so the summary silently fell back to "No
// transcript available to summarize." -- EndWarRoom treats both fetches as
// best-effort (see warroom_service.go), so this never surfaced as a
// visible error, only as an empty-looking summary. All of these are
// user-consentable -- none require a tenant admin to click "Grant admin
// consent" (confirmed against the permissions reference for
// Calendars.ReadWrite specifically), which is the whole point of this flow.
//
// Changing this string means any already-connected account's existing
// refresh token was issued under the OLD scope set and won't cover the
// new one -- reconnect via "Connect Teams account" again after a scope
// change, same as any OAuth app that adds a permission.
const teamsOAuthScopes = "offline_access openid profile User.Read https://graph.microsoft.com/OnlineMeetings.ReadWrite https://graph.microsoft.com/Calendars.ReadWrite https://graph.microsoft.com/OnlineMeetingTranscript.Read.All https://graph.microsoft.com/OnlineMeetingArtifact.Read.All"

// BuildAuthorizeURL builds the Microsoft identity platform authorize URL
// for the delegated Teams OAuth connect flow (see
// services.TeamsOAuthService.InitiateConnect). The browser is sent here;
// Microsoft eventually redirects back to redirectURI with ?code=&state=.
//
// prompt=consent is deliberate and confirmed necessary live: without it,
// Azure AD can silently reuse an existing consent grant recorded for this
// app+user and never show the account an updated consent screen at all --
// including for a scope added to the app registration *after* the user's
// first connect. That silently starves the resulting token of the new
// scope with no error anywhere (Graph then just quietly ignores
// isOnlineMeeting on event creation, which is what "reconnect" kept
// failing to fix before this). Forcing the picker/consent screen every
// time means a scope change always actually re-prompts.
func BuildAuthorizeURL(tenantID, clientID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", redirectURI)
	v.Set("response_mode", "query")
	v.Set("scope", teamsOAuthScopes)
	v.Set("state", state)
	v.Set("prompt", "consent")
	return fmt.Sprintf("%s/%s/oauth2/v2.0/authorize?%s", MicrosoftLoginBaseURL, tenantID, v.Encode())
}

// ExchangeCode exchanges an OAuth authorization code (from the callback's
// ?code= param) for an access token + refresh token pair. redirectURI must
// exactly match what was sent to BuildAuthorizeURL and what's registered on
// the app registration -- Microsoft rejects a mismatch.
func ExchangeCode(ctx context.Context, tenantID, clientID, clientSecret, code, redirectURI string) (accessToken, refreshToken string, err error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	tr, err := postTokenForm(ctx, oauthHTTPClient, tenantID, form)
	if err != nil {
		return "", "", fmt.Errorf("exchange authorization code: %w", err)
	}
	if tr.RefreshToken == "" {
		return "", "", fmt.Errorf("exchange authorization code: no refresh_token in response " +
			"(check that offline_access is in the requested scope and the app registration allows it)")
	}
	return tr.AccessToken, tr.RefreshToken, nil
}

type graphMeResponse struct {
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
}

// FetchMe returns the display email of whichever account accessToken
// belongs to -- used right after ExchangeCode to record which service/bot
// account got connected, for display in the Integrations settings UI.
// Prefers userPrincipalName (always present for a work/school account)
// and falls back to mail (userPrincipalName is sometimes not a real
// mailbox address).
func FetchMe(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GraphBaseURL+"/me?$select=userPrincipalName,mail", nil)
	if err != nil {
		return "", fmt.Errorf("build /me request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch /me: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch /me failed: %d", resp.StatusCode)
	}

	var me graphMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", fmt.Errorf("decode /me response: %w", err)
	}
	if me.UserPrincipalName != "" {
		return me.UserPrincipalName, nil
	}
	return me.Mail, nil
}
