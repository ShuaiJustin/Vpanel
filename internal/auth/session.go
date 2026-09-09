package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
)

// Session is durable login state. Deleting a session revokes both tokens in a pair.
// The password fingerprint invalidates every old session after any password change.
type Session struct {
	ID         string    `gorm:"primaryKey;size:64"`
	UserID     int64     `gorm:"index;not null"`
	Kind       string    `gorm:"size:16;not null"`
	Credential string    `gorm:"size:64;not null"`
	ExpiresAt  time.Time `gorm:"index;not null"`
}

func (Session) TableName() string { return "auth_sessions" }

type SessionStore interface {
	Create(context.Context, *Session) error
	Get(context.Context, string) (*Session, error)
	Consume(context.Context, string) (bool, error)
}

type databaseSessionStore struct{ db *gorm.DB }

// NewDatabaseSessionStore performs the additive migration before auth starts.
func NewDatabaseSessionStore(db *gorm.DB) (SessionStore, error) {
	if db == nil {
		return nil, fmt.Errorf("session database is required")
	}
	if err := db.AutoMigrate(&Session{}); err != nil {
		return nil, err
	}
	return &databaseSessionStore{db: db}, nil
}

func (s *databaseSessionStore) Create(ctx context.Context, session *Session) error {
	// Login writes are infrequent; prune expired sessions here instead of adding a scheduler.
	if err := s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&Session{}).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(session).Error
}
func (s *databaseSessionStore) Get(ctx context.Context, id string) (*Session, error) {
	var session Session
	err := s.db.WithContext(ctx).First(&session, "id = ?", id).Error
	return &session, err
}
func (s *databaseSessionStore) Consume(ctx context.Context, id string) (bool, error) {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&Session{})
	return result.RowsAffected == 1, result.Error
}

// The in-memory default keeps embedded/test use independent of a database.
// Production must inject NewDatabaseSessionStore so revocation survives restarts.
type memorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

func (s *memorySessionStore) Create(_ context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, entry := range s.sessions {
		if time.Now().After(entry.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
	s.sessions[session.ID] = *session
	return nil
}
func (s *memorySessionStore) Get(_ context.Context, id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session revoked")
	}
	return &session, nil
}
func (s *memorySessionStore) Consume(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.sessions[id]
	delete(s.sessions, id)
	return ok, nil
}

func (s *Service) WithSessionStore(store SessionStore) *Service {
	if store != nil {
		s.sessions = store
	}
	return s
}
func (s *Service) WithUserRepository(users repository.UserRepository) *Service {
	s.users = users
	return s
}

func credentialFingerprint(passwordHash string) string {
	sum := sha256.Sum256([]byte(passwordHash))
	return hex.EncodeToString(sum[:])
}

func (s *Service) newSession(ctx context.Context, userID int64, kind string, expiry time.Duration, expectedPassword ...string) (*Session, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid session user")
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	session := &Session{ID: hex.EncodeToString(random), UserID: userID, Kind: kind, ExpiresAt: time.Now().Add(expiry)}
	if s.users != nil {
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !user.Enabled {
			return nil, fmt.Errorf("user disabled")
		}
		if len(expectedPassword) > 0 && expectedPassword[0] != user.PasswordHash {
			return nil, fmt.Errorf("credentials changed during login")
		}
		session.Credential = credentialFingerprint(user.PasswordHash)
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) validateSession(ctx context.Context, id string, userID int64, kind string) error {
	if id == "" || userID <= 0 {
		return fmt.Errorf("missing session")
	}
	session, err := s.sessions.Get(ctx, id)
	if err != nil {
		return err
	}
	if session.UserID != userID || session.Kind != kind || !time.Now().Before(session.ExpiresAt) {
		return fmt.Errorf("invalid session")
	}
	if s.users != nil {
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if !user.Enabled || session.Credential != credentialFingerprint(user.PasswordHash) {
			return fmt.Errorf("credentials changed or user disabled")
		}
	}
	return nil
}

func (s *Service) consumeSession(ctx context.Context, id string) error {
	ok, err := s.sessions.Consume(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session already consumed or revoked")
	}
	return nil
}
