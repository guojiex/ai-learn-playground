package viewmodel

import (
	"encoding/json"

	"ai-learn-playground/affiliate-ai-studio/go/internal/worker"
)

func FromWorkerResponse(sessionID string, resp worker.Response) (map[string]any, error) {
	var payload map[string]any
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &payload); err != nil {
			return nil, err
		}
	} else {
		payload = make(map[string]any)
	}
	payload["session_id"] = sessionID
	return payload, nil
}
