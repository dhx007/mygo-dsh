package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// packageToolTimeout bounds one package tool execution; a var so tests can
// shorten it.
var packageToolTimeout = 30 * time.Second

// RunPackageTool executes an installed plugin package's tool script in a
// fresh goja VM. The script must define `function run(args) { … }`; args is
// the model's parsed argument object. The same bridge functions as PTC mode
// are available (bash/read_file/write_file/glob/grep/web_fetch/web_search).
// A fresh VM per call keeps packages side-effect-free between invocations.
func RunPackageTool(ctx context.Context, script, argsJSON, cwd string, call func(name, argsJSON string) (string, bool)) (string, bool) {
	vm := goja.New()

	bind := func(name string) func(goja.FunctionCall) goja.Value {
		return func(fc goja.FunctionCall) goja.Value {
			out, failed := call(name, toJSONValue(vm, fc.Arguments))
			if failed {
				panic(vm.NewGoError(fmt.Errorf("%s 失败: %s", name, out)))
			}
			return vm.ToValue(out)
		}
	}
	for _, name := range ptcToolNames {
		_ = vm.Set(name+"_raw", bind(name))
	}
	if _, err := vm.RunString(ptcWrapJS); err != nil {
		return "error: 初始化插件运行环境失败: " + err.Error(), true
	}

	timer := time.AfterFunc(packageToolTimeout, func() { vm.Interrupt("插件工具超时（30s）") })
	defer timer.Stop()

	if _, err := vm.RunString(script); err != nil {
		if ie, ok := err.(*goja.InterruptedError); ok {
			_ = ie
			return "error: 插件工具超时（30s）", true
		}
		return "error: 插件脚本加载失败: " + err.Error(), true
	}
	run, ok := goja.AssertFunction(vm.Get("run"))
	if !ok {
		return "error: 插件脚本未定义 function run(args)", true
	}
	argVal, err := vm.RunString("(" + orEmptyObject(argsJSON) + ")")
	if err != nil {
		return "error: 参数不是合法 JSON: " + err.Error(), true
	}
	res, err := run(goja.Undefined(), argVal)
	if err != nil {
		if _, interrupted := err.(*goja.InterruptedError); interrupted {
			return "error: 插件工具超时（30s）", true
		}
		return "error: " + err.Error(), true
	}
	out := ""
	if res != nil && !goja.IsUndefined(res) && !goja.IsNull(res) {
		if s, ok := res.Export().(string); ok {
			out = s
		} else {
			out = res.String()
		}
	}
	if len(out) > 32*1024 {
		out = out[:32*1024] + "…[已截断]"
	}
	if strings.TrimSpace(out) == "" {
		out = "(插件工具执行完毕，无返回值)"
	}
	return out, false
}

func orEmptyObject(argsJSON string) string {
	if strings.TrimSpace(argsJSON) == "" {
		return "{}"
	}
	return argsJSON
}
