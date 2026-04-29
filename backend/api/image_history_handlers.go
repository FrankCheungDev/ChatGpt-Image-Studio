package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"chatgpt2api/internal/config"
	"chatgpt2api/internal/imagehistory"
)

type adminImageConversation struct {
	imagehistory.Conversation
	UserName string `json:"userName,omitempty"`
}

func (s *Server) handleListImageConversations(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	items, err := store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": filterImageConversationsByUser(items, user.ID)})
}

func (s *Server) handleListAllImageConversations(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	items, err := store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	userNames, err := s.imageHistoryUserNamesByID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": adminImageConversations(items, userNames)})
}

func (s *Server) handleGetAnyImageConversation(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	item, err := store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if item == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}
	userNames, err := s.imageHistoryUserNamesByID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": adminImageConversationFor(*item, userNames)})
}

func (s *Server) handleGetImageConversation(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	item, err := store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if item == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}
	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	if item.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}
	if item.DeletedAt != "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleSaveImageConversation(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	var body imagehistory.Conversation
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}
	if pathID := strings.TrimSpace(r.PathValue("id")); pathID != "" {
		body.ID = pathID
	}
	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	body.UserID = user.ID
	body.DeletedAt = ""

	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	item, err := store.Save(r.Context(), body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDeleteImageConversation(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	item, err := store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	if item == nil || item.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}
	if _, err := store.MarkDeleted(r.Context(), r.PathValue("id"), time.Now()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleClearImageConversations(w http.ResponseWriter, r *http.Request) {
	if !s.serverImageConversationStorageEnabled() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "server image storage is disabled"})
		return
	}
	store, err := imagehistory.NewStore(s.cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	user, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "authorization is invalid"})
		return
	}
	items, err := store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	for _, item := range items {
		if item.UserID != user.ID || item.DeletedAt != "" {
			continue
		}
		if _, err := store.MarkDeleted(r.Context(), item.ID, time.Now()); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleImportImageConversations(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items   []imagehistory.Conversation `json:"items"`
		Storage struct {
			Backend                  string `json:"backend"`
			ImageDir                 string `json:"imageDir"`
			SQLitePath               string `json:"sqlitePath"`
			RedisAddr                string `json:"redisAddr"`
			RedisPassword            string `json:"redisPassword"`
			RedisDB                  int    `json:"redisDb"`
			RedisPrefix              string `json:"redisPrefix"`
			ImageConversationStorage string `json:"imageConversationStorage"`
			ImageDataStorage         string `json:"imageDataStorage"`
		} `json:"storage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	tempCfg := config.New(s.cfg.RootDir())
	tempCfg.Storage.Backend = body.Storage.Backend
	tempCfg.Storage.ImageDir = firstNonEmpty(body.Storage.ImageDir, s.cfg.Storage.ImageDir)
	tempCfg.Storage.SQLitePath = firstNonEmpty(body.Storage.SQLitePath, s.cfg.Storage.SQLitePath)
	tempCfg.Storage.RedisAddr = firstNonEmpty(body.Storage.RedisAddr, s.cfg.Storage.RedisAddr)
	tempCfg.Storage.RedisPassword = body.Storage.RedisPassword
	tempCfg.Storage.RedisDB = body.Storage.RedisDB
	tempCfg.Storage.RedisPrefix = firstNonEmpty(body.Storage.RedisPrefix, s.cfg.Storage.RedisPrefix)
	tempCfg.Storage.ImageConversationStorage = firstNonEmpty(body.Storage.ImageConversationStorage, "server")
	tempCfg.Storage.ImageDataStorage = firstNonEmpty(body.Storage.ImageDataStorage, tempCfg.Storage.ImageConversationStorage)

	store, err := imagehistory.NewStore(tempCfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	if err := store.Clear(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	for _, item := range body.Items {
		if _, err := store.Save(r.Context(), item); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(body.Items)})
}

func (s *Server) serverImageConversationStorageEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(s.cfg.Storage.ImageConversationStorage), "server")
}

func filterImageConversationsByUser(items []imagehistory.Conversation, userID string) []imagehistory.Conversation {
	filtered := make([]imagehistory.Conversation, 0, len(items))
	for _, item := range items {
		if item.UserID == userID && item.DeletedAt == "" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *Server) imageHistoryUserNamesByID() (map[string]string, error) {
	userNames := map[string]string{
		legacyAdminUserID: "legacy-admin",
	}
	if s.userStore == nil {
		return userNames, nil
	}
	items, err := s.userStore.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		userNames[item.ID] = item.Username
	}
	return userNames, nil
}

func adminImageConversations(items []imagehistory.Conversation, userNames map[string]string) []adminImageConversation {
	enriched := make([]adminImageConversation, 0, len(items))
	for _, item := range items {
		enriched = append(enriched, adminImageConversationFor(item, userNames))
	}
	return enriched
}

func adminImageConversationFor(item imagehistory.Conversation, userNames map[string]string) adminImageConversation {
	return adminImageConversation{
		Conversation: item,
		UserName:     userNames[item.UserID],
	}
}
