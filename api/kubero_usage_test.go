package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/rca/llm"
)

func TestPostKuberoUsage(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get(HandoffSecretHeader)
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := postKuberoUsage(srv.URL, "handoff-secret", "demo-production", "kubero+u1+demo-production@handoff.local", "openai/gpt-oss-120b", llm.Usage{
		PromptTokens:     80,
		CompletionTokens: 20,
		TotalTokens:      100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if gotAuth != "handoff-secret" {
		t.Errorf("secret header: got %q", gotAuth)
	}
	var body kuberoUsageBody
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body: %s", err)
	}
	if body.Namespace != "demo-production" || body.TotalTokens != 100 || body.Email == "" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestPostKuberoUsageRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	if err := postKuberoUsage(srv.URL, "x", "ns", "", "m", llm.Usage{TotalTokens: 1}); err == nil {
		t.Fatal("expected an error")
	}
}
