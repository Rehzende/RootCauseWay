package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// No test coverage existed for this package at all before this file --
// RBACEnforcer.RequirePermission has been fully implemented (and correct)
// since migration 010, but was never wired onto a single route until now,
// so nothing ever exercised it end-to-end either.

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(method string, permissions map[string][]string, setAuthCtx bool) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/whatever", nil)
	if setAuthCtx {
		c.Set("auth_context", &models.AuthContext{Permissions: permissions})
	}
	return c, w
}

func TestRequirePermission_AllowsExactMatch(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, map[string][]string{"incidents": {"read"}}, true)
	mw := (&RBACEnforcer{}).RequirePermission("incidents", "read")

	mw(c)

	assert.False(t, c.IsAborted())
}

func TestRequirePermission_AllowsActionWildcard(t *testing.T) {
	c, _ := newTestContext(http.MethodDelete, map[string][]string{"incidents": {"*"}}, true)
	mw := (&RBACEnforcer{}).RequirePermission("incidents", "delete")

	mw(c)

	assert.False(t, c.IsAborted())
}

func TestRequirePermission_AllowsResourceWildcard(t *testing.T) {
	c, _ := newTestContext(http.MethodPost, map[string][]string{"*": {"write"}}, true)
	mw := (&RBACEnforcer{}).RequirePermission("anything", "write")

	mw(c)

	assert.False(t, c.IsAborted())
}

func TestRequirePermission_DeniesMissingPermission(t *testing.T) {
	c, w := newTestContext(http.MethodPost, map[string][]string{"incidents": {"read"}}, true)
	mw := (&RBACEnforcer{}).RequirePermission("incidents", "write")

	mw(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_DeniesUnknownResource(t *testing.T) {
	c, w := newTestContext(http.MethodGet, map[string][]string{"incidents": {"read"}}, true)
	mw := (&RBACEnforcer{}).RequirePermission("software", "read")

	mw(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_MissingAuthContext_Returns401(t *testing.T) {
	c, w := newTestContext(http.MethodGet, nil, false)
	mw := (&RBACEnforcer{}).RequirePermission("incidents", "read")

	mw(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireResourcePermission_InfersReadForGET(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, map[string][]string{"incidents": {"read"}}, true)
	mw := (&RBACEnforcer{}).RequireResourcePermission("incidents")

	mw(c)

	assert.False(t, c.IsAborted())
}

func TestRequireResourcePermission_InfersWriteForNonGET(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			c, _ := newTestContext(method, map[string][]string{"incidents": {"write"}}, true)
			mw := (&RBACEnforcer{}).RequireResourcePermission("incidents")

			mw(c)

			assert.False(t, c.IsAborted(), "expected %s to be allowed by incidents:write", method)
		})
	}
}

func TestRequireResourcePermission_ReadOnlyUserBlockedFromWrite(t *testing.T) {
	c, w := newTestContext(http.MethodPost, map[string][]string{"incidents": {"read"}}, true)
	mw := (&RBACEnforcer{}).RequireResourcePermission("incidents")

	mw(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRequireResourcePermission_SameHandlerInstanceDoesNotLeakActionAcrossRequests
// is a regression test: the first implementation declared `action := "write"`
// OUTSIDE the returned closure, so it was computed once when the middleware
// was constructed and then silently reused (and, for a "read" call, never
// reset) across every request through that same gin.HandlerFunc value --
// which is exactly how it's used in cmd/api/main.go (one
// RequireResourcePermission(resource) call, its returned handler wired onto
// one route, invoked fresh per HTTP request). A GET followed by a POST
// against the SAME handler instance must independently get "read" then
// "write", not carry state between calls.
func TestRequireResourcePermission_SameHandlerInstanceDoesNotLeakActionAcrossRequests(t *testing.T) {
	mw := (&RBACEnforcer{}).RequireResourcePermission("incidents")

	getCtx, _ := newTestContext(http.MethodGet, map[string][]string{"incidents": {"read"}}, true)
	mw(getCtx)
	require.False(t, getCtx.IsAborted(), "GET with only read permission should be allowed")

	// Same mw value, now a write request with only read granted -- must be
	// evaluated fresh as "write" and denied, not incorrectly allowed because
	// a captured "read" leaked over from the call above.
	postCtx, w := newTestContext(http.MethodPost, map[string][]string{"incidents": {"read"}}, true)
	mw(postCtx)
	assert.True(t, postCtx.IsAborted(), "POST with only read permission must be denied, not silently allowed by a leaked action")
	assert.Equal(t, http.StatusForbidden, w.Code)
}
