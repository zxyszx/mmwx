package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"miaomiaowu/internal/storage"
)

type SSRFAllowListSettingsHandler struct {
	repo *storage.TrafficRepository
}

func NewSSRFAllowListSettingsHandler(repo *storage.TrafficRepository) *SSRFAllowListSettingsHandler {
	return &SSRFAllowListSettingsHandler{repo: repo}
}

func (h *SSRFAllowListSettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		value, err := h.repo.GetSystemSetting(r.Context(), SSRFAllowListSettingKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"allowed_hosts": value})
	case http.MethodPut:
		var body struct {
			AllowedHosts string `json:"allowed_hosts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeBadRequest(w, "invalid body")
			return
		}
		value := strings.TrimSpace(body.AllowedHosts)
		if _, invalid := parseSSRFAllowList(value); len(invalid) > 0 {
			writeBadRequest(w, "无法识别的白名单项: "+strings.Join(invalid, ", "))
			return
		}
		if err := h.repo.SetSystemSetting(r.Context(), SSRFAllowListSettingKey, value); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		SetSSRFAllowList(value)
		respondJSON(w, http.StatusOK, map[string]bool{"success": true})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}
