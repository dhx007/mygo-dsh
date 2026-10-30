package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

// REQ-011 AC-4: multimodal messages marshal content as an array (image_url
// nested format), round-trip through UnmarshalJSON, and plain messages keep
// the exact string format.
func TestMessageMultimodalMarshal(t *testing.T) {
	m := Message{Role: "user", Parts: []ContentPart{
		{Type: "text", Text: "看这张图"},
		{Type: "image_url", ImageURL: "data:image/png;base64,AAA"},
	}}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, `{"role":"user","content":[`) {
		t.Fatalf("content not an array: %s", s)
	}
	for _, want := range []string{
		`{"text":"看这张图","type":"text"}`,
		`{"image_url":{"url":"data:image/png;base64,AAA"},"type":"image_url"}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("wire format missing %s in %s", want, s)
		}
	}
	var back Message
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Role != "user" || len(back.Parts) != 2 ||
		back.Parts[0].Text != "看这张图" || back.Parts[1].ImageURL != "data:image/png;base64,AAA" {
		t.Fatalf("roundtrip: %+v", back)
	}

	// 纯文本消息：线格式与既有完全一致
	plain, _ := json.Marshal(Message{Role: "user", Content: "hi"})
	if string(plain) != `{"role":"user","content":"hi"}` {
		t.Fatalf("plain format changed: %s", plain)
	}
	var pback Message
	if err := json.Unmarshal(plain, &pback); err != nil || pback.Content != "hi" || len(pback.Parts) != 0 {
		t.Fatalf("plain roundtrip: %+v %v", pback, err)
	}

	// 工具消息（含 ToolCallID）序列化不受影响
	tool := Message{Role: "tool", ToolCallID: "t1", Content: "result"}
	tb, _ := json.Marshal(tool)
	if !strings.Contains(string(tb), `"tool_call_id":"t1"`) {
		t.Fatalf("tool format changed: %s", tb)
	}
}
