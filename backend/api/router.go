package api

import (
	"net/http"

	"chatgpt2api/internal/accounts"
	"chatgpt2api/internal/cliproxy"
	"chatgpt2api/internal/config"
	"chatgpt2api/internal/users"
)

func SetupRouter(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client) http.Handler {
	return NewServer(cfg, store, syncClient).Handler()
}

func SetupRouterWithUsers(cfg *config.Config, store *accounts.Store, syncClient *cliproxy.Client, userStore *users.Store) http.Handler {
	return NewServerWithUsers(cfg, store, syncClient, userStore).Handler()
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
