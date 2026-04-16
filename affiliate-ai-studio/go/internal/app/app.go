package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-learn-playground/affiliate-ai-studio/go/internal/session"
	"ai-learn-playground/affiliate-ai-studio/go/internal/worker"
)

type App struct {
	Title    string
	Worker   worker.Caller
	Sessions *session.Store
}

func New(title string, caller worker.Caller) *App {
	return &App{
		Title:    title,
		Worker:   caller,
		Sessions: session.NewStore(),
	}
}

func (a *App) RunAffiliateFlow(ctx context.Context, payload any) (string, worker.Response, error) {
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	body, err := json.Marshal(payload)
	if err != nil {
		return "", worker.Response{}, fmt.Errorf("marshal run payload: %w", err)
	}
	resp, err := a.Worker.Call(ctx, worker.Request{
		Action:    "run_affiliate_graph",
		SessionID: sessionID,
		Payload:   body,
	})
	if err != nil {
		return sessionID, resp, err
	}
	a.Sessions.Save(sessionID, resp)
	return sessionID, resp, nil
}
