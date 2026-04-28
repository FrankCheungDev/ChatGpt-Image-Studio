package users

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreBootstrapsAdminAndAuthenticatesSessions(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	admin, created, err := store.EnsureBootstrapAdmin("admin", "admin-pass")
	if err != nil {
		t.Fatalf("EnsureBootstrapAdmin() returned error: %v", err)
	}
	if !created {
		t.Fatal("expected first admin to be created")
	}
	if admin.Role != RoleAdmin {
		t.Fatalf("admin role = %q, want %q", admin.Role, RoleAdmin)
	}

	authenticated, err := store.Authenticate("admin", "admin-pass")
	if err != nil {
		t.Fatalf("Authenticate() returned error: %v", err)
	}
	if authenticated.ID != admin.ID {
		t.Fatalf("authenticated ID = %q, want %q", authenticated.ID, admin.ID)
	}
	if _, err := store.Authenticate("admin", "wrong-pass"); err == nil {
		t.Fatal("Authenticate() with wrong password succeeded")
	}

	token, err := store.CreateSession(admin.ID, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	fromSession, err := store.UserBySession(token)
	if err != nil {
		t.Fatalf("UserBySession() returned error: %v", err)
	}
	if fromSession.ID != admin.ID || fromSession.Role != RoleAdmin {
		t.Fatalf("session user = %#v, want admin %#v", fromSession, admin)
	}

	if err := store.DeleteSession(token); err != nil {
		t.Fatalf("DeleteSession() returned error: %v", err)
	}
	if _, err := store.UserBySession(token); err == nil {
		t.Fatal("deleted session still authenticates")
	}
}

func TestStoreCreatesAndUpdatesUsers(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "studio.db"))
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	defer store.Close()

	user, err := store.CreateUser(CreateUserInput{
		Username: "alice",
		Password: "alice-pass",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser() returned error: %v", err)
	}
	if user.Role != RoleUser || user.Disabled {
		t.Fatalf("created user = %#v", user)
	}

	disabled := true
	role := RoleAdmin
	updated, err := store.UpdateUser(user.ID, UpdateUserInput{
		Role:     &role,
		Disabled: &disabled,
	})
	if err != nil {
		t.Fatalf("UpdateUser() returned error: %v", err)
	}
	if updated.Role != RoleAdmin || !updated.Disabled {
		t.Fatalf("updated user = %#v, want admin disabled", updated)
	}
	if _, err := store.Authenticate("alice", "alice-pass"); err == nil {
		t.Fatal("disabled user should not authenticate")
	}

	disabled = false
	if _, err := store.UpdateUser(user.ID, UpdateUserInput{
		Password: "next-pass",
		Disabled: &disabled,
	}); err != nil {
		t.Fatalf("UpdateUser(password) returned error: %v", err)
	}
	if _, err := store.Authenticate("alice", "alice-pass"); err == nil {
		t.Fatal("old password still authenticates")
	}
	if _, err := store.Authenticate("alice", "next-pass"); err != nil {
		t.Fatalf("new password did not authenticate: %v", err)
	}
}
