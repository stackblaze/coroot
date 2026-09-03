package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func completionResponse(content string) string {
	return completionResponseWithUsage(content, 0, 0, 0)
}

func completionResponseWithUsage(content string, prompt, completion, total int) string {
	body := map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"content": content}},
		},
	}
	if total > 0 || prompt+completion > 0 {
		body["usage"] = map[string]any{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      total,
		}
	}
	b, _ := json.Marshal(body)
	return string(b)
}

func TestRCAAnalyzeSuccess(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(completionResponse(`{
			"short_summary": "Checkout errors after deploy",
			"root_cause": "The v2.1 deployment introduced a failing DB migration.",
			"immediate_fixes": "- Roll back to v2.0",
			"detailed_root_cause_analysis": "Errors began 30s after rollout."
		}`)))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseUrl: srv.URL, ApiKey: "sk-test", Model: "test/model"})
	res, err := c.Analyze(context.Background(), map[string]string{"app": "checkout"})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if gotPath != "/chat/completions" {
		t.Errorf("expected /chat/completions, got %s", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("unexpected authorization header: %s", gotAuth)
	}
	if !strings.Contains(gotBody, `"model":"test/model"`) {
		t.Errorf("model missing from request: %s", gotBody)
	}
	if !strings.Contains(gotBody, "checkout") {
		t.Errorf("evidence missing from request: %s", gotBody)
	}
	if res.ShortSummary != "Checkout errors after deploy" {
		t.Errorf("unexpected short summary: %q", res.ShortSummary)
	}
	if !strings.Contains(res.RootCause, "failing DB migration") {
		t.Errorf("unexpected root cause: %q", res.RootCause)
	}
	if res.ImmediateFixes != "- Roll back to v2.0" {
		t.Errorf("unexpected fixes: %q", res.ImmediateFixes)
	}
}

func TestRCAAnalyzeTrailingSlashBaseUrl(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(completionResponse(`{"root_cause":"disk full"}`)))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseUrl: srv.URL + "/", Model: "m"})
	if _, err := c.Analyze(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("expected /chat/completions, got %s", gotPath)
	}
}

func TestRCAAnalyzeStripsCodeFence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(completionResponse("```json\n{\"root_cause\":\"OOMKilled\"}\n```")))
	}))
	defer srv.Close()

	res, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if res.RootCause != "OOMKilled" {
		t.Errorf("unexpected root cause: %q", res.RootCause)
	}
}

func TestRCAAnalyzeDerivesShortSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(completionResponse(`{"root_cause":"Postgres ran out of connections. Latency followed."}`)))
	}))
	defer srv.Close()

	res, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if res.ShortSummary != "Postgres ran out of connections." {
		t.Errorf("unexpected short summary: %q", res.ShortSummary)
	}
}

func TestRCAAnalyzeRetriesInvalidJson(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(completionResponse("I think the database is slow.")))
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "rejected") {
			t.Errorf("retry did not include corrective message: %s", body)
		}
		_, _ = w.Write([]byte(completionResponse(`{"root_cause":"slow query"}`)))
	}))
	defer srv.Close()

	res, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
	if res.RootCause != "slow query" {
		t.Errorf("unexpected root cause: %q", res.RootCause)
	}
}

func TestRCAAnalyzeGivesUpAfterOneRetry(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(completionResponse("still not json")))
	}))
	defer srv.Close()

	_, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRCAAnalyzeMissingRootCause(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(completionResponse(`{"short_summary":"something broke"}`)))
	}))
	defer srv.Close()

	_, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "root_cause") {
		t.Fatalf("expected a root_cause error, got %v", err)
	}
}

func TestRCAAnalyzeUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	_, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("expected the upstream message to surface, got %v", err)
	}
}

func TestRCAAnalyzeCapturesUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(completionResponseWithUsage(`{"root_cause":"OOMKilled"}`, 80, 20, 100)))
	}))
	defer srv.Close()

	res, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if res.Usage.PromptTokens != 80 || res.Usage.CompletionTokens != 20 || res.Usage.TotalTokens != 100 {
		t.Errorf("unexpected usage: %+v", res.Usage)
	}
}

func TestRCAAnalyzeSumsRetryUsage(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(completionResponseWithUsage("not json", 10, 2, 12)))
			return
		}
		_, _ = w.Write([]byte(completionResponseWithUsage(`{"root_cause":"slow query"}`, 11, 3, 14)))
	}))
	defer srv.Close()

	res, err := NewClient(Config{BaseUrl: srv.URL, Model: "m"}).Analyze(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if res.Usage.TotalTokens != 26 {
		t.Errorf("expected retry tokens to sum, got %+v", res.Usage)
	}
}
