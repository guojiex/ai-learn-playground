package capstone

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlatformOutput 是单个平台的文案输出.
type PlatformOutput struct {
	Platform string `json:"platform"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Tags     string `json:"tags"`
	Raw      string `json:"raw"`
}

// ParseOutput 从 agent 输出中解析出标题/正文/标签.
func ParseOutput(platform, raw string) PlatformOutput {
	out := PlatformOutput{
		Platform: platform,
		Name:     PlatformName(platform),
		Raw:      raw,
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err == nil {
		if v, ok := m["title"].(string); ok {
			out.Title = v
		}
		if v, ok := m["body"].(string); ok {
			out.Body = v
		}
		if arr, ok := m["tags"].([]any); ok {
			var tags []string
			for _, t := range arr {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
			out.Tags = strings.Join(tags, " ")
		}
		return out
	}
	// 非 JSON: 整段当 body.
	out.Body = raw
	out.Title = firstLine(raw)
	return out
}

// FormatMarkdown 把多平台输出渲染成 markdown.
func FormatMarkdown(outputs []PlatformOutput) string {
	var b strings.Builder
	for _, o := range outputs {
		fmt.Fprintf(&b, "--- %s ---\n", o.Name)
		if o.Title != "" {
			fmt.Fprintf(&b, "标题: %s\n", o.Title)
		}
		if o.Body != "" {
			fmt.Fprintf(&b, "\n%s\n", o.Body)
		}
		if o.Tags != "" {
			fmt.Fprintf(&b, "\n标签: %s\n", o.Tags)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if len([]rune(s)) > 40 {
		return string([]rune(s)[:40]) + "..."
	}
	return s
}
