// Package llm implements a minimal streaming client for the DeepSeek
// chat-completions API (OpenAI-compatible), including tool calling.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ToolCall is one function the model asked to run.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Message is one chat message, in history and on the wire. Reasoning is
// display-only and never sent back (the API rejects it).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`

	Reasoning string `json:"-"`
}

// ToolDef describes a tool's JSON schema, as the API expects it.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolSpec is a tool entry of a request.
type ToolSpec struct {
	Type     string  `json:"type"`
	Function ToolDef `json:"function"`
}

// Request is a chat-completions request body.
type Request struct {
	Model       string     `json:"model"`
	Messages    []Message  `json:"messages"`
	Tools       []ToolSpec `json:"tools,omitempty"`
	Stream      bool       `json:"stream"`
	Temperature float64    `json:"temperature,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`
}

// StreamHandler receives deltas as they arrive, on the caller's goroutine.
type StreamHandler struct {
	OnContent   func(delta string)
	OnReasoning func(delta string)
}

// Client calls a DeepSeek-compatible API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New builds a client; baseURL defaults to the DeepSeek API.
func New(baseURL, apiKey string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.deepseek.com"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				ForceAttemptHTTP2:     true,
			},
		},
	}
}

type streamEvent struct {
	Choices []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Reasoning string `json:"reasoning_content"`
			ToolCalls []struct {
				Index    *int   `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat streams one completion and returns the assembled assistant message.
// Deltas go to h as they arrive. A canceled context returns the partial
// message and the context error.
func (c *Client) Chat(ctx context.Context, req Request, h StreamHandler) (Message, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Message{}, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	out := Message{Role: "assistant"}
	calls := map[int]*ToolCall{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var evt streamEvent
		if json.Unmarshal([]byte(data), &evt) != nil {
			continue
		}
		if evt.Error != nil && evt.Error.Message != "" {
			return out, fmt.Errorf("API 流错误: %s", evt.Error.Message)
		}
		if len(evt.Choices) == 0 {
			continue
		}
		d := evt.Choices[0].Delta
		if d.Content != "" {
			out.Content += d.Content
			if h.OnContent != nil {
				h.OnContent(d.Content)
			}
		}
		if d.Reasoning != "" {
			out.Reasoning += d.Reasoning
			if h.OnReasoning != nil {
				h.OnReasoning(d.Reasoning)
			}
		}
		for _, tc := range d.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			cur := calls[idx]
			if cur == nil {
				cur = &ToolCall{Type: "function"}
				calls[idx] = cur
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Type != "" {
				cur.Type = tc.Type
			}
			if tc.Function.Name != "" {
				cur.Function.Name = tc.Function.Name
			}
			cur.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return out, fmt.Errorf("读取流失败: %w", err)
	}
	idxs := make([]int, 0, len(calls))
	for i := range calls {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	for _, i := range idxs {
		out.ToolCalls = append(out.ToolCalls, *calls[i])
	}
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, nil
}
