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

// ContentPart is one part of a multimodal content array (REQ-011): text or
// an image reference.
type ContentPart struct {
	Type     string `json:"type"` // text | image_url
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"-"` // 序列化为嵌套 {"image_url":{"url":…}}
}

func (p ContentPart) MarshalJSON() ([]byte, error) {
	switch p.Type {
	case "image_url":
		return json.Marshal(map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": p.ImageURL},
		})
	default:
		return json.Marshal(map[string]any{"type": "text", "text": p.Text})
	}
}

func (p *ContentPart) UnmarshalJSON(b []byte) error {
	var raw struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.Type = raw.Type
	switch raw.Type {
	case "image_url":
		if raw.ImageURL != nil {
			p.ImageURL = raw.ImageURL.URL
		}
	default:
		p.Text = raw.Text
	}
	return nil
}

// Message is one chat message, in history and on the wire. Reasoning is
// display-only and never sent back (the API rejects it). Messages with
// Parts marshal their content as a multimodal array; plain messages keep
// the string format.
type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Parts      []ContentPart `json:"parts,omitempty"` // 本地形态；线格式见 MarshalJSON
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`

	Reasoning string `json:"-"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	wire := struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content,omitempty"`
		ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
	}{Role: m.Role, ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID}
	if len(m.Parts) > 0 {
		b, err := json.Marshal(m.Parts)
		if err != nil {
			return nil, err
		}
		wire.Content = b
	} else if m.Content != "" {
		b, err := json.Marshal(m.Content)
		if err != nil {
			return nil, err
		}
		wire.Content = b
	}
	return json.Marshal(wire)
}

func (m *Message) UnmarshalJSON(b []byte) error {
	var raw struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  []ToolCall      `json:"tool_calls"`
		ToolCallID string          `json:"tool_call_id"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	m.Role, m.ToolCalls, m.ToolCallID = raw.Role, raw.ToolCalls, raw.ToolCallID
	if len(raw.Content) > 0 && string(raw.Content) != "null" {
		if raw.Content[0] == '[' {
			return json.Unmarshal(raw.Content, &m.Parts)
		}
		return json.Unmarshal(raw.Content, &m.Content)
	}
	return nil
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

// StreamOptions asks the provider to include usage in the streaming tail
// chunk (OpenAI-compatible stream_options).
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// Request is a chat-completions request body.
type Request struct {
	Model       string     `json:"model"`
	Messages    []Message  `json:"messages"`
	Tools       []ToolSpec `json:"tools,omitempty"`
	Stream      bool       `json:"stream"`
	Temperature float64    `json:"temperature,omitempty"`
	MaxTokens   int        `json:"max_tokens,omitempty"`

	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
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
	// OnUsage, when set, is called once per finished call with the token
	// usage reported by the provider (REQ-015).
	OnUsage func(Usage)
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

// Usage is the token accounting of one API call, reported via
// Client.OnUsage (REQ-015).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
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
	Usage *Usage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat streams one completion and returns the assembled assistant message.
// Deltas go to h as they arrive. A canceled context returns the partial
// message and the context error.
func (c *Client) Chat(ctx context.Context, req Request, h StreamHandler) (Message, error) {
	req.Stream = true
	req.StreamOptions = &StreamOptions{IncludeUsage: true}
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
	var lastUsage *Usage
	defer func() {
		if lastUsage != nil && c.OnUsage != nil {
			c.OnUsage(*lastUsage)
		}
	}()
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
		if evt.Usage != nil {
			lastUsage = evt.Usage // 尾块：choices 为空但带 usage
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

// ListModels queries the provider's /models endpoint (OpenAI-compatible
// GET) and returns the model ids sorted by name.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpResp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(res.Data))
	for _, m := range res.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// Complete runs one non-streaming chat completion and returns the message
// content — used for cheap auxiliary calls (session titles, REQ-014).
func (c *Client) Complete(ctx context.Context, model, system, user string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 24
	}
	body := Request{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	httpResp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 1024*1024))
	if err != nil {
		return "", err
	}
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &res); err != nil {
		return "", err
	}
	if res.Usage != nil && c.OnUsage != nil {
		c.OnUsage(*res.Usage)
	}
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("无补全结果")
	}
	return res.Choices[0].Message.Content, nil
}
