// Package hitl 实现 Human-in-the-Loop 审批机制 (M8).
//
// 在 agent 执行高风险动作 (如发布商品/改库存) 前暂停, 把决策权交给人类.
// 审批记录持久化到 SQLite, 支持通过 CLI / Web UI 审批后 resume.
package hitl

import (
	"context"
	"fmt"
	"time"

	"ai-learn-playground/agent-lab/internal/store"
)

// Status 是审批的状态机.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusEdited   Status = "edited"
)

// RiskLevel 镜像 tools.RiskLevel, 避免 hitl 反向依赖 tools.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Approval 是一条审批记录.
type Approval struct {
	ID         string    `json:"id"`
	ConvID     string    `json:"conv_id"`
	StepIdx    int       `json:"step_idx"`
	Tool       string    `json:"tool"`
	Args       string    `json:"args"`
	Payload    string    `json:"payload"`
	RiskLevel  RiskLevel `json:"risk_level"`
	Status     Status    `json:"status"`
	Reviewer   string    `json:"reviewer"`
	Note       string    `json:"note"`
	EditedArgs string    `json:"edited_args,omitempty"`
	CreatedAt  int64     `json:"created_at"`
	ReviewedAt int64     `json:"reviewed_at,omitempty"`
}

// Manager 管理审批的创建、查询、审批/拒绝/编辑.
type Manager struct {
	store *store.Store
}

// NewManager 构造一个绑定到 store 的审批管理器.
func NewManager(st *store.Store) *Manager {
	return &Manager{store: st}
}

// Create 创建一条 pending 审批.
func (m *Manager) Create(ctx context.Context, id, convID string, stepIdx int, tool, args, payload string, risk RiskLevel) (*Approval, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	a := &Approval{
		ID:        id,
		ConvID:    convID,
		StepIdx:   stepIdx,
		Tool:      tool,
		Args:      args,
		Payload:   payload,
		RiskLevel: risk,
		Status:    StatusPending,
		CreatedAt: time.Now().Unix(),
	}
	_, err := m.store.DB().ExecContext(ctx,
		`INSERT INTO approvals(id, conv_id, step_idx, tool, args, payload, risk_level, status, created_at)
		 VALUES(?,?,?,?,?,?,?, ?,?)`,
		a.ID, a.ConvID, a.StepIdx, a.Tool, a.Args, a.Payload, string(a.RiskLevel), string(a.Status), a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create approval: %w", err)
	}
	return a, nil
}

// Get 按 ID 查询审批.
func (m *Manager) Get(ctx context.Context, id string) (*Approval, error) {
	row := m.store.DB().QueryRowContext(ctx,
		`SELECT id, conv_id, step_idx, tool, args, payload, risk_level, status, reviewer, note, edited_args, created_at, reviewed_at
		 FROM approvals WHERE id=?`, id)
	return scanApproval(row)
}

// ListPending 返回所有 pending 审批, 按创建时间排序.
func (m *Manager) ListPending(ctx context.Context) ([]Approval, error) {
	return m.listByStatus(ctx, StatusPending)
}

// ListAll 返回全部审批 (含已完成), 按 created_at 降序.
func (m *Manager) ListAll(ctx context.Context, limit int) ([]Approval, error) {
	if m.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := m.store.DB().QueryContext(ctx,
		`SELECT id, conv_id, step_idx, tool, args, payload, risk_level, status, reviewer, note, edited_args, created_at, reviewed_at
		 FROM approvals ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()
	return scanApprovals(rows)
}

// CountPending 返回 pending 审批数 (用于 UI 徽标).
func (m *Manager) CountPending(ctx context.Context) (int, error) {
	if m.store == nil {
		return 0, nil
	}
	var n int
	err := m.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM approvals WHERE status='pending'`).Scan(&n)
	return n, err
}

// Approve 批准审批, 可附带 reviewer 与 note.
func (m *Manager) Approve(ctx context.Context, id, reviewer, note string) (*Approval, error) {
	return m.update(ctx, id, StatusApproved, reviewer, note, "")
}

// Reject 拒绝审批, 拒绝原因通过 note 传给上一步角色.
func (m *Manager) Reject(ctx context.Context, id, reviewer, note string) (*Approval, error) {
	return m.update(ctx, id, StatusRejected, reviewer, note, "")
}

// Edit 批准但修改参数, agent 用 editedArgs 继续执行.
func (m *Manager) Edit(ctx context.Context, id, reviewer, note, editedArgs string) (*Approval, error) {
	return m.update(ctx, id, StatusEdited, reviewer, note, editedArgs)
}

func (m *Manager) update(ctx context.Context, id string, status Status, reviewer, note, editedArgs string) (*Approval, error) {
	if m.store == nil {
		return nil, fmt.Errorf("store not configured")
	}
	now := time.Now().Unix()
	_, err := m.store.DB().ExecContext(ctx,
		`UPDATE approvals SET status=?, reviewer=?, note=?, edited_args=?, reviewed_at=? WHERE id=? AND status='pending'`,
		string(status), reviewer, note, editedArgs, now, id)
	if err != nil {
		return nil, fmt.Errorf("update approval %s: %w", id, err)
	}
	return m.Get(ctx, id)
}

func (m *Manager) listByStatus(ctx context.Context, status Status) ([]Approval, error) {
	if m.store == nil {
		return nil, nil
	}
	rows, err := m.store.DB().QueryContext(ctx,
		`SELECT id, conv_id, step_idx, tool, args, payload, risk_level, status, reviewer, note, edited_args, created_at, reviewed_at
		 FROM approvals WHERE status=? ORDER BY created_at ASC`, string(status))
	if err != nil {
		return nil, fmt.Errorf("list approvals by status: %w", err)
	}
	defer rows.Close()
	return scanApprovals(rows)
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanApproval(row rowScanner) (*Approval, error) {
	var a Approval
	var risk, status string
	err := row.Scan(&a.ID, &a.ConvID, &a.StepIdx, &a.Tool, &a.Args, &a.Payload,
		&risk, &status, &a.Reviewer, &a.Note, &a.EditedArgs, &a.CreatedAt, &a.ReviewedAt)
	if err != nil {
		return nil, err
	}
	a.RiskLevel = RiskLevel(risk)
	a.Status = Status(status)
	return &a, nil
}

func scanApprovals(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]Approval, error) {
	var out []Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}
