package main

// The plugin registry is the single source of truth for "everything is a
// plugin": the plugins page, the settings section, the model-side plugins
// tool and preset consistency checks all read from here. IDs double as the
// enabled_tools keys in config.json, so old configs keep working.

// Plugin is one registered plugin (currently every built-in tool).
type Plugin struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Category    string   `json:"category"`
	Source      string   `json:"source"`
	Tools       []string `json:"tools"`
	Kind        string   `json:"kind"`
}

// pluginGroup is a category slice for the grouped views.
type pluginGroup struct {
	Category string
	Plugins  []Plugin
}

var pluginCategories = []string{"终端", "文件", "搜索", "子代理", "循环", "系统"}

func builtinPlugins() []Plugin {
	return []Plugin{
		{ID: "bash", Name: "Shell 终端", Description: "在持久工作区里执行 bash 命令：构建、测试、git、目录操作", Version: "0.1.0", Category: "终端", Source: "内置", Tools: []string{"bash"}, Kind: "tool"},
		{ID: "read_file", Name: "读文件", Description: "带行号读取文本文件，支持分页", Version: "0.1.0", Category: "文件", Source: "内置", Tools: []string{"read_file"}, Kind: "tool"},
		{ID: "write_file", Name: "写文件", Description: "创建或覆写文本文件，自动建父目录", Version: "0.1.0", Category: "文件", Source: "内置", Tools: []string{"write_file"}, Kind: "tool"},
		{ID: "edit_file", Name: "编辑文件", Description: "精确字符串替换编辑，支持全部替换", Version: "0.1.0", Category: "文件", Source: "内置", Tools: []string{"edit_file"}, Kind: "tool"},
		{ID: "glob", Name: "文件匹配", Description: "按 glob 模式（支持 **）列出文件路径", Version: "0.1.0", Category: "搜索", Source: "内置", Tools: []string{"glob"}, Kind: "tool"},
		{ID: "grep", Name: "内容搜索", Description: "用正则搜索文件内容，返回 路径:行号: 文本", Version: "0.1.0", Category: "搜索", Source: "内置", Tools: []string{"grep"}, Kind: "tool"},
		{ID: "web_fetch", Name: "网页抓取", Description: "抓取网页并抽取可读文本（去标签）", Version: "0.1.0", Category: "搜索", Source: "内置", Tools: []string{"web_fetch"}, Kind: "tool"},
		{ID: "web_search", Name: "网页搜索", Description: "搜索网页并返回标题、链接与摘要", Version: "0.1.0", Category: "搜索", Source: "内置", Tools: []string{"web_search"}, Kind: "tool"},
		{ID: "task", Name: "子代理", Description: "把自包含的子任务委托给一个一层深的子 agent", Version: "0.1.0", Category: "子代理", Source: "内置", Tools: []string{"task"}, Kind: "tool"},
		{ID: "todo", Name: "任务清单", Description: "维护本会话的 plan checklist 卡片", Version: "0.1.0", Category: "循环", Source: "内置", Tools: []string{"todo"}, Kind: "tool"},
		{ID: "run_code", Name: "PTC 脚本", Description: "模型写一段 JavaScript，把多次工具操作合并成一次执行", Version: "0.1.0", Category: "循环", Source: "内置", Tools: []string{"run_code"}, Kind: "tool"},
		{ID: "plugins", Name: "插件管理", Description: "模型侧的插件管理工具：列出、查询、启用、停用插件（创造模式默认挂载）", Version: "0.1.0", Category: "系统", Source: "内置", Tools: []string{"plugins"}, Kind: "control"},
		{ID: "skill", Name: "技能", Description: "SKILL.md 技能包：模型可列出并按需加载技能指令（标准/创造模式默认挂载）", Version: "0.1.0", Category: "系统", Source: "内置", Tools: []string{"skill"}, Kind: "tool"},
		{ID: "mcp", Name: "MCP", Description: "Model Context Protocol 服务器（stdio JSONRPC）：外部工具接入插件页第四段，工具默认走审批门", Version: "0.1.0", Category: "系统", Source: "内置", Tools: []string{"mcp"}, Kind: "control"},
		{ID: "instructions", Name: "工作区指令", Description: "自动加载工作区根目录 AGENTS.md（回退 CLAUDE.md）项目指令并注入系统提示词", Version: "0.1.0", Category: "系统", Source: "内置", Tools: nil, Kind: "context"},
	}
}

// pluginGroups buckets plugins into the fixed category order, dropping
// empty groups.
func pluginGroups(all []Plugin) []pluginGroup {
	index := map[string]int{}
	var groups []pluginGroup
	for _, cat := range pluginCategories {
		index[cat] = len(groups)
		groups = append(groups, pluginGroup{Category: cat})
	}
	for _, p := range all {
		if i, ok := index[p.Category]; ok {
			groups[i].Plugins = append(groups[i].Plugins, p)
		} else {
			groups[index[pluginCategories[len(pluginCategories)-1]]].Plugins = append(
				groups[index[pluginCategories[len(pluginCategories)-1]]].Plugins, p)
		}
	}
	out := groups[:0]
	for _, g := range groups {
		if len(g.Plugins) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func pluginByID(all []Plugin, id string) (Plugin, bool) {
	for _, p := range all {
		if p.ID == id {
			return p, true
		}
	}
	return Plugin{}, false
}
