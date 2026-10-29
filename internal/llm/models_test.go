package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// REQ-010 AC-1: /models returns sorted ids; non-200 reports the status.
func TestListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-reasoner"},{"id":"deepseek-chat"},{"id":""}]}`))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "sk-test")
	list, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0] != "deepseek-chat" || list[1] != "deepseek-reasoner" {
		t.Fatalf("list = %v", list)
	}

	unauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(unauth.Close)
	if _, err := New(unauth.URL, "bad").ListModels(context.Background()); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("401: %v", err)
	}
}
