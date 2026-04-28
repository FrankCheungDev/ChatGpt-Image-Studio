package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"chatgpt2api/internal/accounts"
	"chatgpt2api/internal/users"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if s.userStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "user store is not configured"})
		return
	}
	items, err := s.userStore.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if s.userStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "user store is not configured"})
		return
	}
	var body struct {
		Username string     `json:"username"`
		Password string     `json:"password"`
		Role     users.Role `json:"role"`
		Disabled bool       `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	item, err := s.userStore.CreateUser(users.CreateUserInput{
		Username: body.Username,
		Password: body.Password,
		Role:     body.Role,
		Disabled: body.Disabled,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if s.userStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "user store is not configured"})
		return
	}
	var body struct {
		ID       string      `json:"id"`
		Password string      `json:"password"`
		Role     *users.Role `json:"role"`
		Disabled *bool       `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	id := firstNonEmpty(r.PathValue("id"), body.ID)
	item, err := s.userStore.UpdateUser(id, users.UpdateUserInput{
		Password: body.Password,
		Role:     body.Role,
		Disabled: body.Disabled,
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == users.ErrUserNotFound {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleWorkbenchStatus(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())
	allowDisabled := s.allowDisabledStudioImageAccounts()
	mode := s.configuredImageMode()
	accountsSummary := map[string]any{
		"total":               0,
		"available":           0,
		"availableQuota":      0,
		"hasAvailablePaid":    false,
		"hasUsableFreeLegacy": false,
	}
	if mode == "cpa" {
		accountsSummary["hasAvailablePaid"] = true
	} else if store := s.getStore(); store != nil {
		items, err := store.ListAccounts()
		if err == nil {
			total, available, quota, paid, freeLegacy := summarizeWorkbenchAccounts(items, allowDisabled, mode, s.cfg.ChatGPT.FreeImageRoute)
			accountsSummary["total"] = total
			accountsSummary["available"] = available
			accountsSummary["availableQuota"] = quota
			accountsSummary["hasAvailablePaid"] = paid
			accountsSummary["hasUsableFreeLegacy"] = freeLegacy
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
		"chatgpt": map[string]any{
			"imageMode":                        mode,
			"freeImageRoute":                   s.cfg.ChatGPT.FreeImageRoute,
			"studioAllowDisabledImageAccounts": allowDisabled,
		},
		"accounts": accountsSummary,
		"storage": map[string]any{
			"imageConversationStorage": s.cfg.Storage.ImageConversationStorage,
			"imageDataStorage":         s.cfg.Storage.ImageDataStorage,
		},
	})
}

func summarizeWorkbenchAccounts(items []accounts.PublicAccount, allowDisabled bool, imageMode, freeImageRoute string) (int, int, int, bool, bool) {
	total := len(items)
	available := 0
	quota := 0
	hasPaid := false
	hasFreeLegacy := false
	for _, item := range items {
		if !isImageAccountUsable(item, allowDisabled) {
			continue
		}
		available++
		quota += imageRemainingForSummary(item)
		paid := isPaidImageAccountType(item.Type)
		hasPaid = hasPaid || paid
		hasFreeLegacy = hasFreeLegacy ||
			(strings.EqualFold(imageMode, "studio") &&
				strings.EqualFold(strings.TrimSpace(freeImageRoute), "legacy") &&
				!paid)
	}
	return total, available, quota, hasPaid, hasFreeLegacy
}

func imageRemainingForSummary(account accounts.PublicAccount) int {
	for _, item := range account.LimitsProgress {
		if strings.TrimSpace(stringValue(item["feature_name"])) == "image_gen" {
			if remaining, ok := numberAsInt(item["remaining"]); ok && remaining >= 0 {
				return remaining
			}
		}
	}
	if account.Quota < 0 {
		return 0
	}
	return account.Quota
}

func numberAsInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	default:
		return 0, false
	}
}
