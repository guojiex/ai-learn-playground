package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type runtimeInfo struct {
	Profile         string `json:"profile"`
	BaseURL         string `json:"base_url"`
	RequestModel    string `json:"request_model"`
	BackendOK       bool   `json:"backend_ok"`
	BackendService  string `json:"backend_service,omitempty"`
	BackendModel    string `json:"backend_model,omitempty"`
	ConfiguredModel string `json:"configured_model,omitempty"`
	LoadedModel     string `json:"loaded_model,omitempty"`
	Device          string `json:"device,omitempty"`
	Error           string `json:"error,omitempty"`
}

type backendModelsResp struct {
	Service         string `json:"service"`
	ConfiguredModel string `json:"configured_model"`
	LoadedModel     string `json:"loaded_model"`
	Device          string `json:"device"`
	Data            []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (s *Server) handleRuntimeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info := runtimeInfo{
		Profile:      s.cfg.Profile,
		BaseURL:      s.cfg.BaseURL,
		RequestModel: s.cfg.ModelChat,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	models, err := s.fetchBackendModels(ctx)
	if err != nil {
		info.Error = err.Error()
		writeJSON(w, http.StatusOK, info)
		return
	}
	info.BackendOK = true
	info.BackendService = models.Service
	info.ConfiguredModel = models.ConfiguredModel
	info.LoadedModel = models.LoadedModel
	info.Device = models.Device
	if models.LoadedModel != "" {
		info.BackendModel = models.LoadedModel
	} else if models.ConfiguredModel != "" {
		info.BackendModel = models.ConfiguredModel
	} else if len(models.Data) > 0 {
		info.BackendModel = models.Data[0].ID
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) fetchBackendModels(ctx context.Context) (backendModelsResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.cfg.BaseURL, "/")+"/models", nil)
	if err != nil {
		return backendModelsResp{}, err
	}
	if s.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return backendModelsResp{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return backendModelsResp{}, &runtimeAPIError{StatusCode: resp.StatusCode}
	}
	var out backendModelsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return backendModelsResp{}, err
	}
	return out, nil
}

type runtimeAPIError struct {
	StatusCode int
}

func (e *runtimeAPIError) Error() string {
	return "backend /v1/models returned HTTP " + http.StatusText(e.StatusCode)
}
