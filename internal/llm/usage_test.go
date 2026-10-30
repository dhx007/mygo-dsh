package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// REQ-015 AC-1: streaming calls report the tail-chunk usage and ask the
// provider to include it.
func TestUsageCaptureStreaming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			StreamOptions *struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.StreamOptions == nil || !body.StreamOptions.IncludeUsage {
			t.Errorf("stream_options.include_usage missing")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"hi"}}]}` + "\n\n"))
		f.Flush()
		_, _ = w.Write([]byte(`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}` + "\n\n"))
		f.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		f.Flush()
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "sk-test")
	var got Usage
	calls := 0
	c.OnUsage = func(u Usage) { calls++; got = u }
	_, err := c.Chat(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "x"}}}, StreamHandler{})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if calls != 1 || got.PromptTokens != 10 || got.CompletionTokens != 5 || got.TotalTokens != 15 {
		t.Fatalf("usage: calls=%d %+v", calls, got)
	}
}

// REQ-015 AC-1: non-streaming Complete reports usage too.
func TestUsageCaptureComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "sk-test")
	var got Usage
	c.OnUsage = func(u Usage) { got = u }
	if _, err := c.Complete(context.Background(), "m", "s", "u", 8); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got.PromptTokens != 3 || got.CompletionTokens != 2 {
		t.Fatalf("usage: %+v", got)
	}
}
