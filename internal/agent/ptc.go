package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"

	"mygo-harness/internal/llm"
)

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// RunCodeSpec is the PTC mode's single tool: run a program that calls the
// tools as functions.
func RunCodeSpec() llm.ToolSpec {
	return llm.ToolSpec{
		Type: "function",
		Function: llm.ToolDef{
			Name: "run_code",
			Description: "Run a JavaScript program that calls the tools as plain functions — " +
				"bash(cmd), read_file(path, offset?, limit?), write_file(path, content), glob(pattern, root?), " +
				"grep(pattern, path?, include?), web_fetch(url), web_search(query). " +
				"Merge many operations into one program; return the final answer string.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"code": map[string]any{"type": "string", "description": "JavaScript source; runs inside a function, use return."},
				},
				"required": []string{"code"},
			},
		},
	}
}

var ptcToolNames = []string{"bash", "read_file", "write_file", "glob", "grep", "web_fetch", "web_search"}

// runPTCCode executes the model's JavaScript in a fresh interpreter with
// the tools bound as plain functions, PTC style: many tool operations in
// one round trip. An explicit return (the code runs inside a function)
// or the last expression's value becomes the result.
func runPTCCode(ctx context.Context, code, cwd string, call func(name, argsJSON string) (string, bool)) (string, bool) {
	vm := goja.New()

	bind := func(name string) func(goja.FunctionCall) goja.Value {
		raw := name + "_raw"
		return func(fc goja.FunctionCall) goja.Value {
			out, failed := call(raw, toJSONValue(vm, fc.Arguments))
			if failed {
				panic(vm.NewGoError(fmt.Errorf("%s 失败: %s", name, out)))
			}
			return vm.ToValue(out)
		}
	}
	for _, name := range ptcToolNames {
		_ = vm.Set(name+"_raw", bind(name))
	}

	// Positional-friendly wrappers over the raw JSON-schema bindings.
	wrapJS := `
function bash(cmd) { return bash_raw({command: String(cmd)}); }
function read_file(path, offset_line, limit) {
  var a = {path: String(path)};
  if (offset_line !== undefined && offset_line !== null) a.offset_line = offset_line;
  if (limit !== undefined && limit !== null) a.limit = limit;
  return read_file_raw(a);
}
function write_file(path, content) { return write_file_raw({path: String(path), content: String(content)}); }
function glob(pattern, root) { var a = {pattern: String(pattern)}; if (root) a.path = String(root); return glob_raw(a); }
function grep(pattern, path, include) {
  var a = {pattern: String(pattern)};
  if (path) a.path = String(path);
  if (include) a.include = String(include);
  return grep_raw(a);
}
function web_fetch(url) { return web_fetch_raw({url: String(url)}); }
function web_search(query) { return web_search_raw({query: String(query)}); }
`
	if _, err := vm.RunString(wrapJS); err != nil {
		return "error: 初始化 PTC 环境失败: " + err.Error(), true
	}

	// Hard timeout: interrupt the interpreter.
	timer := time.AfterFunc(120*time.Second, func() { vm.Interrupt("PTC 超时（120s）") })
	defer timer.Stop()

	src := "(function(){\n" + code + "\n})()"
	v, err := vm.RunString(src)
	if err != nil {
		if _, interrupted := err.(*goja.InterruptedError); interrupted {
			return "error: PTC 超时（120s）——把任务拆成更小的程序", true
		}
		return "error: " + err.Error(), true
	}
	out := ""
	if v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		export := v.Export()
		if s, ok := export.(string); ok {
			out = s
		} else if b, merr := jsonMarshal(export); merr == nil {
			out = string(b)
		} else {
			out = v.String()
		}
	}
	if strings.TrimSpace(out) == "" {
		out = "(程序执行完毕，无返回值；请让 run_code 显式 return 结果)"
	}
	if len(out) > 32*1024 {
		out = out[:32*1024] + "…[已截断]"
	}
	return out, false
}

// toJSONValue serializes goja call arguments into the JSON string the Go
// tools expect (one object argument).
func toJSONValue(vm *goja.Runtime, args []goja.Value) string {
	if len(args) == 0 {
		return "{}"
	}
	jsonObj := vm.Get("JSON").ToObject(vm)
	fn, ok := goja.AssertFunction(jsonObj.Get("stringify"))
	if !ok {
		return "{}"
	}
	v, err := fn(goja.Undefined(), args[0])
	if err != nil {
		return "{}"
	}
	return v.String()
}
