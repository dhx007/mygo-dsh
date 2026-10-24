package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatStreamsAndAssemblesToolCalls(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","content":"先看","reasoning_content":"想一想"}}]}`,
		`data: {"choices":[{"delta":{"content":"文件\n"}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash","arguments":"{\"co"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"mmand\":\"ls\"}"}}]}}]}`,
		`data: {"choices":[{"finish_reason":"tool_calls","delta":{}}]}`,
		"data: [DONE]",
		"",
	}, "\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer srv.Close()

	var content, reasoning strings.Builder
	msg, err := New(srv.URL, "sk-test").Chat(context.Background(), Request{
		Model:    "deepseek-chat",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, StreamHandler{
		OnContent:   func(d string) { content.WriteString(d) },
		OnReasoning: func(d string) { reasoning.WriteString(d) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if content.String() != "先看文件\n" {
		t.Errorf("content = %q", content.String())
	}
	if reasoning.String() != "想一想" {
		t.Errorf("reasoning = %q", reasoning.String())
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_1" || msg.ToolCalls[0].Function.Name != "bash" {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
	if msg.ToolCalls[0].Function.Arguments != `{"command":"ls"}` {
		t.Errorf("arguments = %q", msg.ToolCalls[0].Function.Arguments)
	}
}

func TestChatAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"Authentication Fails"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "bad").Chat(context.Background(), Request{
		Model:    "deepseek-chat",
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, StreamHandler{})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got %v", err)
	}
}

func TestReasoningNeverSerialized(t *testing.T) {
	m := Message{Role: "assistant", Content: "ok", Reasoning: "secret"}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "reasoning") {
		t.Errorf("reasoning leaked into wire format: %s", b)
	}
}
