package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mygo-harness/internal/llm"
)

// REQ-014: Complete parses a non-streaming completion and reports errors.
func TestCompleteParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] == true {
			t.Errorf("Complete must not stream")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"登录功能实现"}}]}`))
	}))
	t.Cleanup(srv.Close)

	c := llm.New(srv.URL, "sk-test")
	out, err := c.Complete(context.Background(), "deepseek-chat", "sys", "user", 24)
	if err != nil || strings.TrimSpace(out) != "登录功能实现" {
		t.Fatalf("out=%q err=%v", out, err)
	}

	unauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(unauth.Close)
	if _, err := llm.New(unauth.URL, "bad").Complete(context.Background(), "m", "s", "u", 24); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("401: %v", err)
	}
}
