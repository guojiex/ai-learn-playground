package web

import "fmt"

// NavItem 描述左侧导航的一项.
type NavItem struct {
	Path     string
	Label    string
	Icon     string
	Disabled bool
	Active   bool
}

// navItems 给模板 FuncMap 调用. enabled 标记哪些路径已实装 (非 disabled).
func navItems(active string, enabled map[string]bool) []NavItem {
	defs := []NavItem{
		{Path: "/chat", Label: "Chat", Icon: "chat"},
		{Path: "/tools", Label: "Tools", Icon: "tool", Disabled: true},
		{Path: "/memory", Label: "Memory", Icon: "memory", Disabled: true},
		{Path: "/plan", Label: "Plan", Icon: "plan", Disabled: true},
		{Path: "/multi", Label: "Multi-Agent", Icon: "multi", Disabled: true},
		{Path: "/approvals", Label: "Approvals", Icon: "approve", Disabled: true},
		{Path: "/traces", Label: "Traces", Icon: "trace", Disabled: true},
		{Path: "/router", Label: "Router", Icon: "route", Disabled: true},
		{Path: "/settings", Label: "Settings", Icon: "gear"},
		{Path: "/tutorial", Label: "Tutorial", Icon: "book"},
	}
	for i := range defs {
		if enabled[defs[i].Path] {
			defs[i].Disabled = false
		}
		if defs[i].Path == active {
			defs[i].Active = true
		}
	}
	return defs
}

// Placeholder 描述一个尚未实装的面板.
type Placeholder struct {
	Path      string
	Label     string
	Milestone string
	Note      string
}

func placeholders() []Placeholder {
	return []Placeholder{
		{"/tools", "Tools", "M2", "工具列表与最近调用. 在 M2 启用."},
		{"/memory", "Memory", "M4", "长期记忆 KV 浏览. 在 M4 启用."},
		{"/plan", "Plan", "M6", "Plan DAG 与执行进度. 在 M6 启用."},
		{"/multi", "Multi-Agent", "M7", "Researcher / Writer / Critic 消息流. 在 M7 启用."},
		{"/approvals", "Approvals", "M8", "HITL 待办与详情. 在 M8 启用."},
		{"/traces", "Traces", "M9", "Trace 时间线. 在 M9 启用."},
		{"/router", "Router", "M10", "模型路由命中统计. 在 M10 启用."},
		{"/settings", "Settings", "M0", "运行时配置只读视图."},
	}
}

var _ = fmt.Sprintf
