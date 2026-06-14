package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ReActAction 是 ReAct 协议中的 action 字段.
type ReActAction struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ReActParsed 是 ReAct 协议解析后的结构化输出.
// 如果 Final 非空, 表示本次是最终回答, 不再调用工具.
// 如果 Action 非空, 表示要调用工具.
type ReActParsed struct {
	Thought string       `json:"thought"`
	Final   string       `json:"final,omitempty"`
	Action  *ReActAction `json:"action,omitempty"`
}

// ParseReAct 从模型的原始文本输出中提取 ReAct 协议 JSON.
//
// 容错行为 (按优先级):
//  1. 文本直接是 JSON (最理想).
//  2. 被 ```json ... ``` 代码块包裹.
//  3. 被 ``` ... ``` 代码块包裹 (未标注 json).
//  4. 含有 "{" 和 "}" 的行 (取第一对 {} 包含的内容).
//  5. 中文/英文单引号 ' 替换为 " 后重试 (仅对明显 JSON 特征文本).
//  6. 以上均失败 -> 返回 error, 让主循环走兜底流程.
func ParseReAct(raw string) (*ReActParsed, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, fmt.Errorf("empty model output")
	}

	// 候选 JSON 片段列表, 每个候选都会尝试解析, 第一组成功即使用.
	candidates := make([]string, 0, 6)

	// 1. 整块就是 JSON
	if strings.HasPrefix(text, "{") {
		candidates = append(candidates, text)
	}

	// 2. ```json ... ```
	if s, ok := extractFenced(text, "```json", "```"); ok {
		candidates = append(candidates, s)
	}

	// 3. ``` ... ``` (未标注语言)
	if s, ok := extractFenced(text, "```", "```"); ok {
		candidates = append(candidates, s)
	}

	// 4. 找最外层的 {...}, 从第一个 '{' 到最后一个 '}'.
	if brace, ok := extractFirstBracePair(text); ok {
		candidates = append(candidates, brace)
	}

	// 对每个候选做一次 "单引号 -> 双引号" 的宽容尝试.
	for _, c := range candidates {
		if out, err := tryParse(c); err == nil {
			return out, nil
		}
		if out, err := tryParse(normalizeSingleQuotes(c)); err == nil {
			return out, nil
		}
	}

	return nil, fmt.Errorf("cannot parse react output: %s", truncateForError(text, 120))
}

// tryParse 对一段候选文本做 JSON 解析, 同时校验协议字段.
func tryParse(s string) (*ReActParsed, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty candidate")
	}
	var out ReActParsed
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	// 协议要求: 必须有 final 或 action, 否则不是合法的 ReAct 输出.
	hasFinal := strings.TrimSpace(out.Final) != ""
	hasAction := out.Action != nil && strings.TrimSpace(out.Action.Name) != ""
	if !hasFinal && !hasAction {
		return nil, fmt.Errorf("missing final and action")
	}
	if out.Action != nil && len(out.Action.Args) == 0 {
		out.Action.Args = json.RawMessage("{}")
	}
	return &out, nil
}

// extractFenced 从文本中提取第一次出现的 begin/end 之间的内容.
func extractFenced(text, begin, end string) (string, bool) {
	idx := strings.Index(text, begin)
	if idx < 0 {
		return "", false
	}
	rest := text[idx+len(begin):]
	// 允许 begin 后面紧跟换行.
	rest = strings.TrimLeft(rest, " \t\r\n")
	endIdx := strings.Index(rest, end)
	if endIdx < 0 {
		// 没有闭合的 fence, 尝试把 rest 当成 JSON 使用.
		if strings.Contains(rest, "{") {
			return strings.TrimSpace(rest), true
		}
		return "", false
	}
	return strings.TrimSpace(rest[:endIdx]), true
}

// extractFirstBracePair 从文本中找到最外层的 { ... }, 从第一个 '{' 到最后一个 '}'.
func extractFirstBracePair(text string) (string, bool) {
	open := strings.Index(text, "{")
	if open < 0 {
		return "", false
	}
	// 从后向前找第一个 '}', 但跳过可能出现在字符串里的字符.
	closeIdx := strings.LastIndex(text, "}")
	if closeIdx <= open {
		return "", false
	}
	return strings.TrimSpace(text[open : closeIdx+1]), true
}

// normalizeSingleQuotes 把 JSON 里的单引号替换成双引号,
// 注意: 只替换那些明显作为键/值边界的单引号, 避免误伤 "it's" 这类文本.
//
// 这里采用保守策略: 仅当单引号出现在以下位置时替换:
//   - 紧跟 '{', ',', '}' 或 ':'
//   - 紧跟空白
//   - 在文本最开头/结尾
// 这样能覆盖 1.8B 模型常输出的 "{'name':'foo'}" 这种格式.
func normalizeSingleQuotes(s string) string {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	for i, r := range runes {
		if r == '\'' {
			prev := rune(0)
			next := rune(0)
			if i > 0 {
				prev = runes[i-1]
			}
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			// 前一个字符是 {,},:, 逗号或空白, 或者后一个字符是 }/:/,/空白,
			// 就把 ' 当作边界符, 替换为 ".
			if isBoundaryPrev(prev) || isBoundaryNext(next) || i == 0 || i == len(runes)-1 {
				out = append(out, '"')
				continue
			}
		}
		out = append(out, r)
	}
	return string(out)
}

func isBoundaryPrev(r rune) bool {
	switch r {
	case '{', '}', ':', ',', '[', ']':
		return true
	}
	return unicode.IsSpace(r)
}

func isBoundaryNext(r rune) bool {
	switch r {
	case '{', '}', ':', ',', '[', ']':
		return true
	}
	return unicode.IsSpace(r)
}

func truncateForError(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
