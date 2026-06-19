package agent

import (
	"encoding/json"
	"testing"
)

func TestPlan_Validate_OK(t *testing.T) {
	plan := &Plan{
		Goal: "test",
		Tasks: []Task{
			{ID: "t1", Name: "a", Tool: "kb_search", Args: json.RawMessage(`{}`)},
			{ID: "t2", Name: "b", Depends: []string{"t1"}, Agent: "writer", Prompt: "write"},
		},
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestPlan_Validate_DuplicateID(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "t1", Tool: "x"},
			{ID: "t1", Tool: "y"},
		},
	}
	if err := plan.Validate(); err == nil {
		t.Fatal("expected error for duplicate id")
	}
}

func TestPlan_Validate_UnknownDep(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "t1", Depends: []string{"t99"}, Tool: "x"},
		},
	}
	if err := plan.Validate(); err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestPlan_Validate_Cycle(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "t1", Depends: []string{"t2"}, Tool: "x"},
			{ID: "t2", Depends: []string{"t1"}, Tool: "y"},
		},
	}
	if err := plan.Validate(); err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestPlan_Validate_NoToolOrAgent(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "t1", Name: "a"},
		},
	}
	if err := plan.Validate(); err == nil {
		t.Fatal("expected error for task without tool/agent")
	}
}

func TestPlan_ReadyTasks(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "t1", Tool: "a"},
			{ID: "t2", Tool: "b", Depends: []string{"t1"}},
			{ID: "t3", Tool: "c", Depends: []string{"t1"}},
			{ID: "t4", Agent: "writer", Depends: []string{"t2", "t3"}},
		},
	}
	done := map[string]bool{}
	ready := plan.ReadyTasks(done)
	if len(ready) != 1 || ready[0] != "t1" {
		t.Fatalf("first ready should be [t1], got %v", ready)
	}
	done["t1"] = true
	ready = plan.ReadyTasks(done)
	if len(ready) != 2 {
		t.Fatalf("after t1, should have 2 ready, got %v", ready)
	}
	done["t2"] = true
	done["t3"] = true
	ready = plan.ReadyTasks(done)
	if len(ready) != 1 || ready[0] != "t4" {
		t.Fatalf("after t2+t3, should have [t4], got %v", ready)
	}
	done["t4"] = true
	ready = plan.ReadyTasks(done)
	if len(ready) != 0 {
		t.Fatalf("all done, should have 0 ready, got %v", ready)
	}
}

func TestPlan_TopoLevels(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "t1", Tool: "a"},
			{ID: "t2", Tool: "b", Depends: []string{"t1"}},
			{ID: "t3", Tool: "c", Depends: []string{"t1"}},
			{ID: "t4", Agent: "w", Depends: []string{"t2", "t3"}},
		},
	}
	levels := plan.TopoLevels()
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}
	if len(levels[0]) != 1 || levels[0][0] != "t1" {
		t.Fatalf("level 0 should be [t1], got %v", levels[0])
	}
	if len(levels[1]) != 2 {
		t.Fatalf("level 1 should have 2 nodes, got %v", levels[1])
	}
	if len(levels[2]) != 1 || levels[2][0] != "t4" {
		t.Fatalf("level 2 should be [t4], got %v", levels[2])
	}
}

func TestParsePlan_Valid(t *testing.T) {
	raw := `{"goal":"test","tasks":[{"id":"t1","name":"a","tool":"x","args":{}}]}`
	plan, err := parsePlan(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Goal != "test" || len(plan.Tasks) != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestParsePlan_Fenced(t *testing.T) {
	raw := "```json\n{\"goal\":\"g\",\"tasks\":[{\"id\":\"t1\",\"tool\":\"x\"}]}\n```"
	plan, err := parsePlan(raw)
	if err != nil {
		t.Fatalf("parse fenced: %v", err)
	}
	if plan.Goal != "g" {
		t.Fatalf("goal=%s", plan.Goal)
	}
}

func TestParsePlan_WithExtraText(t *testing.T) {
	raw := "好的, 这是计划:\n{\"goal\":\"g\",\"tasks\":[{\"id\":\"t1\",\"tool\":\"x\"}]}\n完成."
	plan, err := parsePlan(raw)
	if err != nil {
		t.Fatalf("parse with text: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(plan.Tasks))
	}
}
