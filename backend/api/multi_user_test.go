package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"chatgpt2api/internal/accounts"
	"chatgpt2api/internal/config"
	"chatgpt2api/internal/imagehistory"
	"chatgpt2api/internal/users"
)

func newMultiUserTestServer(t *testing.T) (*Server, *users.Store, users.User, users.User) {
	t.Helper()
	rootDir := t.TempDir()
	cfg := config.New(rootDir)
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	cfg.App.AuthKey = "legacy-admin-secret"
	cfg.App.APIKey = "external-image-secret"
	cfg.Storage.Backend = "sqlite"
	cfg.Storage.SQLitePath = "data/studio.sqlite"
	cfg.Storage.ImageDir = "data/images"
	cfg.Storage.ImageConversationStorage = "server"
	cfg.Storage.ImageDataStorage = "server"

	accountStore, err := accounts.NewStore(cfg)
	if err != nil {
		t.Fatalf("accounts.NewStore() returned error: %v", err)
	}
	t.Cleanup(func() { _ = accountStore.Close() })

	userStore, err := users.NewStore(filepath.Join(rootDir, "data", "studio.sqlite"))
	if err != nil {
		t.Fatalf("users.NewStore() returned error: %v", err)
	}
	t.Cleanup(func() { _ = userStore.Close() })

	admin, _, err := userStore.EnsureBootstrapAdmin("admin", "admin-pass")
	if err != nil {
		t.Fatalf("EnsureBootstrapAdmin() returned error: %v", err)
	}
	alice, err := userStore.CreateUser(users.CreateUserInput{
		Username: "alice",
		Password: "alice-pass",
		Role:     users.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser(alice) returned error: %v", err)
	}

	return NewServerWithUsers(cfg, accountStore, nil, userStore), userStore, admin, alice
}

func loginCookie(t *testing.T, server *Server, username, password string) *http.Cookie {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("login did not set %s cookie", sessionCookieName)
	return nil
}

func doJSON(t *testing.T, server *Server, method, path string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Marshal() returned error: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func TestLoginMeLogoutAndAdminGuard(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	userCookie := loginCookie(t, server, "alice", "alice-pass")

	rec := doJSON(t, server, http.MethodGet, "/auth/me", userCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/auth/me status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"username":"alice"`) || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("/auth/me returned unexpected body: %s", rec.Body.String())
	}

	rec = doJSON(t, server, http.MethodGet, "/api/users", userCookie, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ordinary user /api/users status = %d, want 403", rec.Code)
	}
	rec = doJSON(t, server, http.MethodGet, "/api/users", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/users status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, server, http.MethodPost, "/auth/logout", userCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/auth/logout status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, server, http.MethodGet, "/auth/me", userCookie, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logged out /auth/me status = %d, want 401", rec.Code)
	}
}

func TestConfigResponseIsAdminOnlyAndRedacted(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	server.cfg.CPA.BaseURL = "https://secret-cpa.example"
	server.cfg.CPA.APIKey = "cpa-secret"
	server.cfg.Sync.BaseURL = "https://secret-sync.example"
	server.cfg.Sync.ManagementKey = "sync-secret"
	server.cfg.Proxy.Enabled = true
	server.cfg.Proxy.URL = "socks5h://proxy-secret.example:1080"
	server.cfg.Storage.RedisAddr = "redis-secret.example:6379"
	server.cfg.Storage.RedisPassword = "redis-secret"
	server.cfg.NewAPI.Username = "newapi-user"
	server.cfg.NewAPI.Password = "newapi-secret"
	server.cfg.NewAPI.AccessToken = "newapi-token"
	server.cfg.NewAPI.SessionCookie = "newapi-cookie"
	server.cfg.Sub2API.Email = "sub2api@example.com"
	server.cfg.Sub2API.Password = "sub2api-secret"
	server.cfg.Sub2API.APIKey = "sub2api-key"

	userCookie := loginCookie(t, server, "alice", "alice-pass")
	rec := doJSON(t, server, http.MethodGet, "/api/config", userCookie, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ordinary user /api/config status = %d, want 403", rec.Code)
	}

	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec = doJSON(t, server, http.MethodGet, "/api/config", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/config status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{
		"legacy-admin-secret",
		"external-image-secret",
		"https://secret-cpa.example",
		"cpa-secret",
		"https://secret-sync.example",
		"sync-secret",
		"proxy-secret.example",
		"redis-secret.example",
		"redis-secret",
		"newapi-user",
		"newapi-secret",
		"newapi-token",
		"newapi-cookie",
		"sub2api@example.com",
		"sub2api-secret",
		"sub2api-key",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("config response leaked %q in %s", secret, body)
		}
	}
	if !strings.Contains(body, `"imageConversationStorage":"server"`) {
		t.Fatalf("redacted config lost non-sensitive storage mode: %s", body)
	}
}

func TestAdminAccountResponsesDoNotExposeAccessTokens(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	secretToken := "sk-account-secret-token"
	if _, _, err := server.getStore().AddAccounts([]string{secretToken}); err != nil {
		t.Fatalf("AddAccounts() returned error: %v", err)
	}

	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec := doJSON(t, server, http.MethodGet, "/api/accounts", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/accounts status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, secretToken) {
		t.Fatalf("account response leaked access token in %s", body)
	}
	if strings.Contains(body, `"access_token":"`+secretToken+`"`) {
		t.Fatalf("account response returned raw access_token in %s", body)
	}
}

func TestDiagnosticsExportRedactsEndpointSensitiveFields(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	server.cfg.ChatGPT.ImageMode = "cpa"
	server.cfg.App.AuthKey = "diag-auth-secret"
	server.cfg.App.APIKey = "diag-api-secret"
	server.cfg.CPA.BaseURL = "https://diag-cpa-secret.example"
	server.cfg.CPA.APIKey = "diag-cpa-key"
	server.cfg.Sync.BaseURL = "https://diag-sync-secret.example"
	server.cfg.Sync.ManagementKey = "diag-sync-key"
	server.cfg.Proxy.URL = "socks5h://diag-proxy-user:diag-proxy-pass@diag-proxy-secret.example:1080"
	server.cfg.Storage.RedisAddr = "diag-redis-secret.example:6379"
	server.cfg.Storage.RedisPassword = "diag-redis-password"
	server.cfg.NewAPI.BaseURL = "https://diag-newapi-secret.example"
	server.cfg.NewAPI.Username = "diag-newapi-user"
	server.cfg.NewAPI.Password = "diag-newapi-password"
	server.cfg.NewAPI.AccessToken = "diag-newapi-token"
	server.cfg.NewAPI.SessionCookie = "diag-newapi-cookie"
	server.cfg.Sub2API.BaseURL = "https://diag-sub2api-secret.example"
	server.cfg.Sub2API.Email = "diag-sub2api@example.com"
	server.cfg.Sub2API.Password = "diag-sub2api-password"
	server.cfg.Sub2API.APIKey = "diag-sub2api-key"
	server.cfg.Sub2API.GroupID = "diag-sub2api-group"

	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec := doJSON(t, server, http.MethodGet, "/api/diagnostics/export", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/diagnostics/export status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{
		"diag-auth-secret",
		"diag-api-secret",
		"diag-cpa-secret.example",
		"diag-cpa-key",
		"diag-sync-secret.example",
		"diag-sync-key",
		"diag-proxy-user",
		"diag-proxy-pass",
		"diag-proxy-secret.example",
		"diag-redis-secret.example",
		"diag-redis-password",
		"diag-newapi-secret.example",
		"diag-newapi-user",
		"diag-newapi-password",
		"diag-newapi-token",
		"diag-newapi-cookie",
		"diag-sub2api-secret.example",
		"diag-sub2api@example.com",
		"diag-sub2api-password",
		"diag-sub2api-key",
		"diag-sub2api-group",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("diagnostics export leaked %q in %s", secret, body)
		}
	}
}

func TestNewAPITokenDiscoverDoesNotReturnGeneratedToken(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	generatedToken := "generated-newapi-secret-token"
	newAPIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/token" {
			t.Fatalf("unexpected NewAPI path %q", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    generatedToken,
		})
	}))
	defer newAPIServer.Close()

	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec := doJSON(t, server, http.MethodPost, "/api/integration/newapi/token", adminCookie, map[string]any{
		"newapi": map[string]any{
			"baseUrl":     newAPIServer.URL,
			"accessToken": "input-newapi-token",
			"userId":      7,
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/integration/newapi/token status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, generatedToken) {
		t.Fatalf("token discover response leaked generated token in %s", body)
	}
}

func TestRequestLogInterfacesRedactSensitiveErrorText(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	server.cfg.ChatGPT.ImageMode = "cpa"
	server.cfg.CPA.BaseURL = "https://log-cpa-secret.example"
	server.cfg.CPA.APIKey = "log-cpa-key"
	server.cfg.Sync.BaseURL = "https://log-sync-secret.example"
	server.cfg.Sync.ManagementKey = "log-sync-key"
	server.cfg.Proxy.URL = "socks5h://log-proxy-secret.example:1080"
	server.cfg.NewAPI.AccessToken = "log-newapi-token"
	server.cfg.Sub2API.APIKey = "log-sub2api-key"
	server.reqLogs.add(imageRequestLogEntry{
		StartedAt:  "2026-04-26T00:00:00Z",
		FinishedAt: "2026-04-26T00:00:01Z",
		Endpoint:   "/v1/images/generations",
		Operation:  "generate",
		Success:    false,
		Error:      "failed https://log-cpa-secret.example using log-cpa-key via socks5h://log-proxy-secret.example:1080 and log-newapi-token/log-sub2api-key",
	})

	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	for _, path := range []string{"/api/requests", "/api/runtime/status", "/api/diagnostics/export"} {
		rec := doJSON(t, server, http.MethodGet, path, adminCookie, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("admin %s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, secret := range []string{
			"log-cpa-secret.example",
			"log-cpa-key",
			"log-sync-secret.example",
			"log-sync-key",
			"log-proxy-secret.example",
			"log-newapi-token",
			"log-sub2api-key",
		} {
			if strings.Contains(body, secret) {
				t.Fatalf("%s leaked %q in %s", path, secret, body)
			}
		}
	}
}

func TestIntegrationProbeDoesNotEchoSensitiveEndpointInput(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec := doJSON(t, server, http.MethodPost, "/api/integration/test", adminCookie, map[string]any{
		"source": "cpa",
		"cpa": map[string]any{
			"baseUrl": "https://integration-cpa-secret.example",
			"apiKey":  "integration-cpa-key",
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/integration/test status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{"integration-cpa-secret.example", "integration-cpa-key"} {
		if strings.Contains(body, secret) {
			t.Fatalf("integration probe leaked %q in %s", secret, body)
		}
	}
}

func TestProxyTestDoesNotEchoSensitiveProxyURL(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec := doJSON(t, server, http.MethodPost, "/api/proxy/test", adminCookie, map[string]any{
		"url": "socks5h://proxy-user:proxy-pass@proxy-test-secret.example:1080",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin /api/proxy/test status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, secret := range []string{"proxy-user", "proxy-pass", "proxy-test-secret.example"} {
		if strings.Contains(body, secret) {
			t.Fatalf("proxy test leaked %q in %s", secret, body)
		}
	}
}

func TestWorkbenchStatusInCPAModeDoesNotRequireLocalAccounts(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	server.cfg.ChatGPT.ImageMode = "cpa"
	server.cfg.CPA.BaseURL = "https://cpa.example"
	server.cfg.CPA.APIKey = "cpa-key"

	userCookie := loginCookie(t, server, "alice", "alice-pass")
	rec := doJSON(t, server, http.MethodGet, "/api/workbench/status", userCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/workbench/status status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		ChatGPT struct {
			ImageMode string `json:"imageMode"`
		} `json:"chatgpt"`
		Accounts struct {
			Total               int  `json:"total"`
			Available           int  `json:"available"`
			AvailableQuota      int  `json:"availableQuota"`
			HasAvailablePaid    bool `json:"hasAvailablePaid"`
			HasUsableFreeLegacy bool `json:"hasUsableFreeLegacy"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() returned error: %v", err)
	}
	if payload.ChatGPT.ImageMode != "cpa" {
		t.Fatalf("imageMode = %q, want cpa", payload.ChatGPT.ImageMode)
	}
	if payload.Accounts.Total != 0 || payload.Accounts.Available != 0 {
		t.Fatalf("account counts = total %d available %d, want empty local pool", payload.Accounts.Total, payload.Accounts.Available)
	}
	if !payload.Accounts.HasAvailablePaid {
		t.Fatal("CPA direct mode should allow paid-resolution presets without local paid accounts")
	}
	if payload.Accounts.HasUsableFreeLegacy {
		t.Fatal("CPA direct mode should not report usable Free legacy accounts")
	}
}

func TestCPASyncEndpointsAreDirectModeNoOp(t *testing.T) {
	server, _, _, _ := newMultiUserTestServer(t)
	server.cfg.ChatGPT.ImageMode = "cpa"
	server.cfg.CPA.BaseURL = "https://cpa.example"
	server.cfg.CPA.APIKey = "cpa-key"
	server.cfg.Sync.Enabled = true
	server.cfg.Sync.BaseURL = "http://127.0.0.1:1"
	server.cfg.Sync.ManagementKey = "legacy-management-key"
	server.syncClient = server.buildSyncClientFromConfig()

	adminCookie := loginCookie(t, server, "admin", "admin-pass")
	rec := doJSON(t, server, http.MethodGet, "/api/sync/status?source=cpa", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("CPA sync status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var status sourceSyncStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Unmarshal(status) returned error: %v", err)
	}
	if !status.Configured {
		t.Fatal("CPA direct status should be configured when CPA image API is configured")
	}
	if status.PullSupported || status.PushSupported {
		t.Fatalf("CPA sync pull/push = %v/%v, want disabled no-op", status.PullSupported, status.PushSupported)
	}
	if status.Local != 0 || status.Remote != 0 {
		t.Fatalf("CPA sync counts = local %d remote %d, want zero", status.Local, status.Remote)
	}
	if !strings.Contains(strings.Join(status.Notes, "\n"), "直连") {
		t.Fatalf("CPA sync notes = %#v, want direct-mode note", status.Notes)
	}

	rec = doJSON(t, server, http.MethodPost, "/api/sync/run", adminCookie, map[string]any{
		"source":    "cpa",
		"direction": "pull",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("CPA sync run = %d, body = %s", rec.Code, rec.Body.String())
	}
	var runPayload struct {
		Result sourceSyncRunResult `json:"result"`
		Status sourceSyncStatus    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &runPayload); err != nil {
		t.Fatalf("Unmarshal(run) returned error: %v", err)
	}
	if !runPayload.Result.OK || runPayload.Result.Imported != 0 || runPayload.Result.Exported != 0 || runPayload.Result.Failed != 0 {
		t.Fatalf("CPA sync run result = %+v, want successful zero-work no-op", runPayload.Result)
	}
	if runPayload.Status.PullSupported || runPayload.Status.PushSupported {
		t.Fatalf("CPA sync status after run pull/push = %v/%v, want disabled no-op", runPayload.Status.PullSupported, runPayload.Status.PushSupported)
	}
}

func TestImageConversationsAreScopedToCurrentUser(t *testing.T) {
	server, userStore, _, alice := newMultiUserTestServer(t)
	bob, err := userStore.CreateUser(users.CreateUserInput{
		Username: "bob",
		Password: "bob-pass",
		Role:     users.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser(bob) returned error: %v", err)
	}
	aliceCookie := loginCookie(t, server, "alice", "alice-pass")
	bobCookie := loginCookie(t, server, "bob", "bob-pass")
	adminCookie := loginCookie(t, server, "admin", "admin-pass")

	conversationBody := func(id, userID string) imagehistory.Conversation {
		return imagehistory.Conversation{
			ID:        id,
			UserID:    userID,
			Title:     "生成",
			Mode:      "generate",
			Prompt:    id,
			Model:     "gpt-image-2",
			Count:     1,
			CreatedAt: "2026-04-26T00:00:00Z",
			Status:    "success",
			Turns: []imagehistory.Turn{{
				ID:        id + "-turn",
				Title:     "生成",
				Mode:      "generate",
				Prompt:    id,
				Model:     "gpt-image-2",
				Count:     1,
				CreatedAt: "2026-04-26T00:00:00Z",
				Status:    "success",
			}},
		}
	}
	rec := doJSON(t, server, http.MethodPut, "/api/image/conversations/alice-conv", aliceCookie, conversationBody("alice-conv", bob.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("alice save status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, server, http.MethodPut, "/api/image/conversations/bob-conv", bobCookie, conversationBody("bob-conv", alice.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("bob save status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, server, http.MethodGet, "/api/image/conversations", aliceCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("alice list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listPayload struct {
		Items []imagehistory.Conversation `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("Unmarshal(alice list) returned error: %v", err)
	}
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != "alice-conv" || listPayload.Items[0].UserID != alice.ID {
		t.Fatalf("alice list = %#v, want only alice-conv owned by alice", listPayload.Items)
	}

	rec = doJSON(t, server, http.MethodGet, "/api/image/conversations/bob-conv", aliceCookie, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("alice get bob status = %d, want 404", rec.Code)
	}
	rec = doJSON(t, server, http.MethodDelete, "/api/image/conversations/bob-conv", aliceCookie, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("alice delete bob status = %d, want 404", rec.Code)
	}

	rec = doJSON(t, server, http.MethodGet, "/api/admin/image/conversations", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin platform history status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var adminPayload struct {
		Items []imagehistory.Conversation `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &adminPayload); err != nil {
		t.Fatalf("Unmarshal(admin list) returned error: %v", err)
	}
	seen := map[string]string{}
	for _, item := range adminPayload.Items {
		seen[item.ID] = item.UserID
	}
	if seen["alice-conv"] != alice.ID || seen["bob-conv"] != bob.ID {
		t.Fatalf("admin platform history = %#v, want both user-owned histories", adminPayload.Items)
	}

	rec = doJSON(t, server, http.MethodGet, "/api/admin/image/conversations/bob-conv", adminCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin platform history detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var detailPayload struct {
		Item imagehistory.Conversation `json:"item"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detailPayload); err != nil {
		t.Fatalf("Unmarshal(admin detail) returned error: %v", err)
	}
	if detailPayload.Item.ID != "bob-conv" || detailPayload.Item.UserID != bob.ID {
		t.Fatalf("admin platform detail = %#v, want bob-conv owned by bob", detailPayload.Item)
	}

	rec = doJSON(t, server, http.MethodGet, "/api/admin/image/conversations/bob-conv", bobCookie, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ordinary user platform history detail status = %d, want 403", rec.Code)
	}
}
