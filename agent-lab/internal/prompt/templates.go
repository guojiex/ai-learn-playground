package prompt

import "fmt"

// QuestionPrompt 把用户的原始输入包成统一格式的 user prompt, 方便后续提取槽位.
//
// 格式很轻: 在消息前面加一个简短的任务前缀, 让模型知道接下来要写文案.
func QuestionPrompt(original string) string {
	if original == "" {
		return ""
	}
	return fmt.Sprintf("[请以台湾电商文案助理的身份回应]\n%s", original)
}

// StyleHint 返回一组风格关键词的提示, 用于插在 system prompt 末尾.
// 传入空字符串返回空, 便于在 system prompt 末尾有条件拼接.
func StyleHint(style string) string {
	style = trimAndLower(style)
	if style == "" {
		return ""
	}
	return fmt.Sprintf("\n\n当前风格: %s", style)
}

func trimAndLower(s string) string {
	s = replaceAll(s)
	if len(s) == 0 {
		return ""
	}
	return s
}

func replaceAll(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\r' || b == '\n' || b == '\t' {
			out = append(out, ' ')
			continue
		}
		out = append(out, b)
	}
	return trimBytes(out)
}

func trimBytes(b []byte) string {
	start := 0
	end := len(b)
	for start < end && b[start] == ' ' {
		start++
	}
	for end > start && b[end-1] == ' ' {
		end--
	}
	return string(b[start:end])
}
