package api

import (
	"net/url"
	"strings"

	"chatgpt2api/internal/accounts"
)

func redactPublicAccounts(items []accounts.PublicAccount) []accounts.PublicAccount {
	if len(items) == 0 {
		return nil
	}
	out := make([]accounts.PublicAccount, 0, len(items))
	for _, item := range items {
		out = append(out, redactPublicAccount(item))
	}
	return out
}

func redactPublicAccount(item accounts.PublicAccount) accounts.PublicAccount {
	item.AccessToken = ""
	return item
}

func redactRefreshErrors(items []accounts.RefreshError) []accounts.RefreshError {
	if len(items) == 0 {
		return nil
	}
	out := make([]accounts.RefreshError, 0, len(items))
	for _, item := range items {
		item.AccessToken = ""
		out = append(out, item)
	}
	return out
}

func (s *Server) redactRequestLogs(items []imageRequestLogEntry) []imageRequestLogEntry {
	if len(items) == 0 {
		return nil
	}
	out := make([]imageRequestLogEntry, 0, len(items))
	for _, item := range items {
		item.Error = s.redactSensitiveText(item.Error)
		out = append(out, item)
	}
	return out
}

func (s *Server) redactSensitiveText(value string) string {
	if strings.TrimSpace(value) == "" || s == nil || s.cfg == nil {
		return value
	}
	return redactInputSensitiveText(value, s.sensitiveResponseValues()...)
}

func redactInputSensitiveText(value string, values ...string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	redacted := value
	for _, secret := range appendURLHosts(dedupeNonEmptyStrings(values)) {
		if !isSensitiveRedactionValue(secret) {
			continue
		}
		redacted = strings.ReplaceAll(redacted, secret, "[redacted]")
	}
	return redacted
}

func (s *Server) sensitiveResponseValues() []string {
	if s == nil || s.cfg == nil {
		return nil
	}
	values := []string{
		s.cfg.App.APIKey,
		s.cfg.App.AuthKey,
		s.cfg.Storage.RedisAddr,
		s.cfg.Storage.RedisPassword,
		s.cfg.Sync.BaseURL,
		s.cfg.Sync.ManagementKey,
		s.cfg.Proxy.URL,
		s.cfg.CPA.BaseURL,
		s.cfg.CPA.APIKey,
		s.cfg.NewAPI.BaseURL,
		s.cfg.NewAPI.Username,
		s.cfg.NewAPI.Password,
		s.cfg.NewAPI.AccessToken,
		s.cfg.NewAPI.SessionCookie,
		s.cfg.Sub2API.BaseURL,
		s.cfg.Sub2API.Email,
		s.cfg.Sub2API.Password,
		s.cfg.Sub2API.APIKey,
		s.cfg.Sub2API.GroupID,
	}
	values = appendURLHosts(values)
	return dedupeNonEmptyStrings(values)
}

func appendURLHosts(values []string) []string {
	for _, value := range append([]string(nil), values...) {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		if host := parsed.Hostname(); strings.TrimSpace(host) != "" {
			values = append(values, host)
		}
	}
	return values
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func isSensitiveRedactionValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 6 {
		return false
	}
	lower := strings.ToLower(trimmed)
	return lower != "localhost" &&
		lower != "127.0.0.1" &&
		lower != "::1" &&
		!strings.HasPrefix(lower, "127.")
}
