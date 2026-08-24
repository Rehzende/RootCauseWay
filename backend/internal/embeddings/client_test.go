package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeVector(n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(i%10) / 10
	}
	return v
}

func embeddingsHandler(t *testing.T, vec []float32, wantModel string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/embeddings", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req embeddingRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, wantModel, req.Model)
		assert.NotEmpty(t, req.Input)

		resp := map[string]interface{}{
			"data": []map[string]interface{}{{"embedding": vec}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func TestClient_Embed_Success(t *testing.T) {
	vec := makeVector(Dimension)
	srv := httptest.NewServer(embeddingsHandler(t, vec, "text-embedding-3-small"))
	defer srv.Close()

	c := New(srv.URL, "test-key", "")
	got, err := c.Embed(context.Background(), "database connection timeout")
	require.NoError(t, err)
	assert.Len(t, got, Dimension)
	assert.InDelta(t, vec[1], got[1], 1e-6)
}

func TestClient_Embed_CustomModel(t *testing.T) {
	srv := httptest.NewServer(embeddingsHandler(t, makeVector(Dimension), "custom-embed-model"))
	defer srv.Close()

	c := New(srv.URL, "test-key", "custom-embed-model")
	_, err := c.Embed(context.Background(), "some text")
	require.NoError(t, err)
}

func TestClient_Embed_EmptyInput(t *testing.T) {
	c := New("http://localhost:1", "k", "")
	_, err := c.Embed(context.Background(), "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty input")
}

func TestClient_Embed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "")
	_, err := c.Embed(context.Background(), "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 429")
}

func TestClient_Embed_WrongDimension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{{"embedding": makeVector(8)}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "")
	_, err := c.Embed(context.Background(), "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dimension")
}

func TestClient_Embed_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "")
	_, err := c.Embed(context.Background(), "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embeddings")
}

func TestClient_Embed_TruncatesLongInput(t *testing.T) {
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embeddingRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotLen = len(req.Input)
		resp := map[string]interface{}{
			"data": []map[string]interface{}{{"embedding": makeVector(Dimension)}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "")
	_, err := c.Embed(context.Background(), strings.Repeat("a", maxInputChars+500))
	require.NoError(t, err)
	assert.Equal(t, maxInputChars, gotLen)
}

func TestNewFromConfig_DisabledWhenNoBase(t *testing.T) {
	e := NewFromConfig("", "key", "model")
	assert.Nil(t, e)
}

func TestNewFromConfig_EnabledWithBase(t *testing.T) {
	e := NewFromConfig("https://api.openai.com/v1", "key", "")
	assert.NotNil(t, e)
}

func TestVectorString(t *testing.T) {
	s := VectorString([]float32{0.5, -1, 2.25})
	assert.Equal(t, "[0.5,-1,2.25]", s)
}
