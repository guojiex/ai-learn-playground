package agent

import (
	"context"
	"strings"
	"testing"

	"ai-learn-playground/agent-lab/internal/llm"
	"ai-learn-playground/agent-lab/internal/tools"
)

func reactResp(content string) llm.ChatResponse {
	return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: content}}
}

// ---------- ParseReAct: 纯 JSON ----------
func TestParseReAct_DirectJSON(t *testing.T) {
	out, err := ParseReAct(`{"thought":"我先查 sku","action":{"name":"product_lookup","args":{"id":"sku_001"}}}`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Action == nil || out.Action.Name != "product_lookup" {
		t.Fatalf("bad action: %+v", out.Action)
	}
	if out.Final != "" {
		t.Fatalf("final should be empty, got %q", out.Final)
	}
}

func TestParseReAct_FinalConverges(t *testing.T) {
	out, err := ParseReAct(`{"thought":"ok","final":"yes"}`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Final != "yes" {
		t.Fatalf("unexpected final: %q", out.Final)
	}
	if out.Action != nil {
		t.Fatalf("action should be nil, got %+v", out.Action)
	}
}

// ---------- ParseReAct: 代码块包裹 ----------
func TestParseReAct_CodeBlockJSON(t *testing.T) {
	fence := "```"
	raw := "\n" + fence + "json\n" + `{"thought":"用 echo","action":{"name":"echo","args":{"k":"v"}}}` + "\n" + fence + "\n"
	out, err := ParseReAct(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Action == nil || out.Action.Name != "echo" {
		t.Fatalf("bad: %+v", out)
	}
}

func TestParseReAct_CodeBlockNoLang(t *testing.T) {
	fence := "```"
	raw := fence + "\n" + `{"thought":"x","final":"hello"}` + "\n" + fence
	out, err := ParseReAct(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Final != "hello" {
		t.Fatalf("unexpected final: %q", out.Final)
	}
}

// ---------- ParseReAct: 前后有多余文字 ----------
func TestParseReAct_WithExtraText(t *testing.T) {
	raw := "好的, 让我查一下 sku.\n\n" + `{"thought":"查","action":{"name":"look","args":{}}}` + "\n以上是我的输出."
	out, err := ParseReAct(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Action == nil || out.Action.Name != "look" {
		t.Fatalf("bad: %+v", out.Action)
	}
}

// ---------- ParseReAct: 单引号 ----------
func TestParseReAct_SingleQuoteJSON(t *testing.T) {
	out, err := ParseReAct(`{'thought':'x', 'final':'done'}`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Final != "done" {
		t.Fatalf("unexpected final: %q", out.Final)
	}
}

// ---------- ParseReAct: 空/乱 ----------
func TestParseReAct_EmptyOrGarbage(t *testing.T) {
	if _, err := ParseReAct(""); err == nil {
		t.Fatal("expected err for empty")
	}
	if _, err := ParseReAct("Hello world, no json here"); err == nil {
		t.Fatal("expected err for garbage")
	}
	if _, err := ParseReAct(`{"thought":"only thought"}`); err == nil {
		t.Fatal("expected err for bare thought")
	}
}

// ---------- ReActAgent.Run: 直接 final ----------
func TestReActAgent_DirectFinal(t *testing.T) {
	client := &stubClient{responses: []llm.ChatResponse{
		reactResp(`{"thought":"好","final":"done"}`),
	}}
	a := NewReActAgent(client, tools.NewRegistry(), Options{SystemPrompt: "x", Model: "m"})
	res, err := a.Run(context.Background(), "hi")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final != "done" {
		t.Fatalf("unexpected final: %q", res.Final)
	}
	if res.Mode != "react" {
		t.Fatalf("unexpected mode: %s", res.Mode)
	}
}

// ---------- ReActAgent.Run: 调用一次工具后收敛 ----------
func TestReActAgent_OneToolThenFinal(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(echoTool{name: "echo"})
	client := &stubClient{responses: []llm.ChatResponse{
		reactResp(`{"thought":"call echo","action":{"name":"echo","args":{"a":1}}}`),
		reactResp(`{"thought":"got it","final":"answer is here"}`),
	}}
	a := NewReActAgent(client, reg, Options{SystemPrompt: "x", Model: "m"})
	res, err := a.Run(context.Background(), "run")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final != "answer is here" {
		t.Fatalf("unexpected final: %q", res.Final)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(res.Steps))
	}
	if res.Steps[0].Kind != StepAction || res.Steps[0].ActionName != "echo" {
		t.Fatalf("bad first step: %+v", res.Steps[0])
	}
	if !strings.Contains(res.Steps[0].Observation, "a") {
		t.Fatalf("bad observation: %q", res.Steps[0].Observation)
	}
}

// ---------- ReActAgent.Run: 未知工具 → 观察到 error 后收敛 ----------
func TestReActAgent_UnknownToolThenRecover(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(echoTool{name: "echo"})
	client := &stubClient{responses: []llm.ChatResponse{
		reactResp(`{"thought":"calling","action":{"name":"unknown","args":{}}}`),
		reactResp(`{"thought":"oops","final":"fallback"}`),
	}}
	a := NewReActAgent(client, reg, Options{SystemPrompt: "x", Model: "m"})
	res, err := a.Run(context.Background(), "run")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final != "fallback" {
		t.Fatalf("unexpected final: %q", res.Final)
	}
	if !strings.Contains(res.Steps[0].Observation, "error") {
		t.Fatalf("expected observation to contain error marker: %q", res.Steps[0].Observation)
	}
}

// ---------- ReActAgent.Run: 解析失败两次 → 降级 ----------
func TestReActAgent_ParseFailDegrade(t *testing.T) {
	reg := tools.NewRegistry()
	client := &stubClient{responses: []llm.ChatResponse{
		reactResp("我不能帮你写文案."),
		reactResp("好的我再说一遍."),
	}}
	a := NewReActAgent(client, reg, Options{SystemPrompt: "x", Model: "m"})
	res, err := a.Run(context.Background(), "run")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Final != "好的我再说一遍." {
		t.Fatalf("expected degraded text: %q", res.Final)
	}
	kindCount := map[StepKind]int{}
	for _, s := range res.Steps {
		kindCount[s.Kind]++
	}
	if kindCount[StepParseRetry] < 1 || kindCount[StepParseDegrade] < 1 {
		t.Fatalf("unexpected step kind counts: %+v", kindCount)
	}
}

// ---------- ReActAgent.Run: MaxSteps 守护 ----------
func TestReActAgent_MaxStepsGuard(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(echoTool{name: "echo"})
	loop := reactResp(`{"thought":"keep going","action":{"name":"echo","args":{}}}`)
	client := &stubClient{responses: []llm.ChatResponse{loop, loop, loop, loop}}
	a := NewReActAgent(client, reg, Options{SystemPrompt: "x", Model: "m", MaxSteps: 3})
	if _, err := a.Run(context.Background(), "keep-going"); err == nil {
		t.Fatal("expected err on max steps")
	}
}
