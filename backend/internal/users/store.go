package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"chatgpt2api/internal/sqliteutil"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrUserNotFound       = errors.New("user not found")
)

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      Role   `json:"role"`
	Disabled  bool   `json:"disabled"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type CreateUserInput struct {
	Username string
	Password string
	Role     Role
	Disabled bool
}

type UpdateUserInput struct {
	Password string
	Role     *Role
	Disabled *bool
}

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("user store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return nil, err
	}
	db, err := sqliteutil.Open(trimmed)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.Init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Init() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("user store is not initialized")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			disabled INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS user_sessions (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) EnsureBootstrapAdmin(username, password string) (User, bool, error) {
	count, err := s.CountUsers()
	if err != nil {
		return User{}, false, err
	}
	if count > 0 {
		admin, err := s.firstAdmin()
		if err == nil {
			return admin, false, nil
		}
		return User{}, false, err
	}
	user, err := s.CreateUser(CreateUserInput{
		Username: firstNonEmpty(username, "admin"),
		Password: firstNonEmpty(password, "admin"),
		Role:     RoleAdmin,
	})
	if err != nil {
		return User{}, false, err
	}
	return user, true, nil
}

func (s *Store) CountUsers() (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, disabled, created_at, updated_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		user, err := scanPublicUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(input CreateUserInput) (User, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return User{}, fmt.Errorf("username is required")
	}
	if strings.TrimSpace(input.Password) == "" {
		return User{}, fmt.Errorf("password is required")
	}
	role := normalizeRole(input.Role)
	hash, err := hashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	user := User{
		ID:        uuid.NewString(),
		Username:  username,
		Role:      role,
		Disabled:  input.Disabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = s.db.Exec(
		`INSERT INTO users(id, username, password_hash, role, disabled, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		user.ID,
		user.Username,
		hash,
		string(user.Role),
		boolInt(user.Disabled),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) UpdateUser(id string, input UpdateUserInput) (User, error) {
	current, err := s.UserByID(id)
	if err != nil {
		return User{}, err
	}
	role := current.Role
	if input.Role != nil {
		role = normalizeRole(*input.Role)
	}
	disabled := current.Disabled
	if input.Disabled != nil {
		disabled = *input.Disabled
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(input.Password) != "" {
		hash, err := hashPassword(input.Password)
		if err != nil {
			return User{}, err
		}
		_, err = s.db.Exec(
			`UPDATE users SET password_hash = ?, role = ?, disabled = ?, updated_at = ? WHERE id = ?`,
			hash,
			string(role),
			boolInt(disabled),
			updatedAt,
			current.ID,
		)
	} else {
		_, err = s.db.Exec(
			`UPDATE users SET role = ?, disabled = ?, updated_at = ? WHERE id = ?`,
			string(role),
			boolInt(disabled),
			updatedAt,
			current.ID,
		)
	}
	if err != nil {
		return User{}, err
	}
	return s.UserByID(current.ID)
}

func (s *Store) UserByID(id string) (User, error) {
	row := s.db.QueryRow(`SELECT id, username, role, disabled, created_at, updated_at FROM users WHERE id = ?`, strings.TrimSpace(id))
	user, err := scanPublicUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

func (s *Store) Authenticate(username, password string) (User, error) {
	var user User
	var passwordHash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, role, disabled, created_at, updated_at FROM users WHERE username = ?`,
		strings.TrimSpace(username),
	).Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &user.Disabled, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if user.Disabled {
		return User{}, ErrUserDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Store) CreateSession(userID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	if _, err := s.UserByID(userID); err != nil {
		return "", err
	}
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO user_sessions(token_hash, user_id, expires_at, created_at, last_seen_at) VALUES(?, ?, ?, ?, ?)`,
		hashSessionToken(token),
		userID,
		now.Add(ttl).Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) UserBySession(token string) (User, error) {
	hash := hashSessionToken(token)
	if hash == "" {
		return User{}, ErrInvalidCredentials
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := s.db.QueryRow(
		`SELECT u.id, u.username, u.role, u.disabled, u.created_at, u.updated_at
		 FROM user_sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`,
		hash,
		now,
	)
	user, err := scanPublicUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if user.Disabled {
		return User{}, ErrUserDisabled
	}
	_, _ = s.db.Exec(`UPDATE user_sessions SET last_seen_at = ? WHERE token_hash = ?`, now, hash)
	return user, nil
}

func (s *Store) DeleteSession(token string) error {
	if hash := hashSessionToken(token); hash != "" {
		_, err := s.db.Exec(`DELETE FROM user_sessions WHERE token_hash = ?`, hash)
		return err
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE expires_at <= ?`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) firstAdmin() (User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, role, disabled, created_at, updated_at FROM users WHERE role = ? ORDER BY created_at ASC LIMIT 1`,
		string(RoleAdmin),
	)
	user, err := scanPublicUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

type publicUserScanner interface {
	Scan(dest ...any) error
}

func scanPublicUser(scanner publicUserScanner) (User, error) {
	var user User
	var role string
	var disabled int
	if err := scanner.Scan(&user.ID, &user.Username, &role, &disabled, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return User{}, err
	}
	user.Role = normalizeRole(Role(role))
	user.Disabled = disabled != 0
	return user, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func newSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashSessionToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

func normalizeRole(role Role) Role {
	if strings.EqualFold(string(role), string(RoleAdmin)) {
		return RoleAdmin
	}
	return RoleUser
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
