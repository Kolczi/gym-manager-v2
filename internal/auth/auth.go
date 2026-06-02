package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gym-manager-v2/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("nieprawidłowy login lub hasło")
	ErrNotAuthenticated   = errors.New("nie zalogowano")
)

type contextKey string

const UserKey contextKey = "user"

// Session stores minimal info in cookie
type Session struct {
	UserID    int64
	Login     string
	Name      string
	Role      string
	ExpiresAt time.Time
}

type Service struct {
	queries   *store.Queries
	sessions  map[string]*Session // token -> session (in-memory, good enough for single-instance)
	cookieName string
}

func NewService(queries *store.Queries) *Service {
	return &Service{
		queries:    queries,
		sessions:   make(map[string]*Session),
		cookieName: "gym_session",
	}
}

// HashPassword creates a bcrypt hash
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Authenticate checks login/password, supports legacy SHA256 with auto-upgrade to bcrypt
func (s *Service) Authenticate(ctx context.Context, login, password string) (*Session, string, error) {
	user, err := s.queries.GetUserByLogin(ctx, login)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Try bcrypt first
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Fallback: legacy SHA256 (no salt)
		sha := sha256.Sum256([]byte(password))
		shaHex := hex.EncodeToString(sha[:])
		if user.PasswordHash != shaHex {
			return nil, "", ErrInvalidCredentials
		}
		// Auto-upgrade to bcrypt
		newHash, _ := HashPassword(password)
		_ = s.queries.UpdateUserPassword(ctx, store.UpdateUserPasswordParams{
			ID:           user.ID,
			PasswordHash: newHash,
		})
	}

	// Create session
	token := generateToken()
	session := &Session{
		UserID:    user.ID,
		Login:     user.Login,
		Name:      user.Name,
		Role:      user.Role,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	s.sessions[token] = session
	return session, token, nil
}

func (s *Service) Logout(token string) {
	delete(s.sessions, token)
}

// Middleware protects routes — redirects to /login if not authenticated
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cookieName)
		if err != nil {
			fmt.Printf("[AUTH] %s %s — no cookie: %v\n", r.Method, r.URL.Path, err)
			s.redirectLogin(w, r)
			return
		}

		session, ok := s.sessions[cookie.Value]
		if !ok || time.Now().After(session.ExpiresAt) {
			fmt.Printf("[AUTH] %s %s — invalid/expired session (found=%v)\n", r.Method, r.URL.Path, ok)
			if ok {
				delete(s.sessions, cookie.Value)
			}
			s.redirectLogin(w, r)
			return
		}

		fmt.Printf("[AUTH] %s %s — OK user=%s\n", r.Method, r.URL.Path, session.Login)
		ctx := context.WithValue(r.Context(), UserKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) redirectLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	SoftRedirect(w, "/login")
}

// SoftRedirect renders a small HTML page that redirects via JS + meta refresh.
// Wails WebView doesn't follow HTTP 302 redirects, so we avoid them entirely.
func SoftRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><script>document.cookie;window.location.href=%q;</script></head><body></body></html>`, url)
}

func (s *Service) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

func (s *Service) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (s *Service) CookieName() string {
	return s.cookieName
}

// UserFromContext extracts current user session from context
func UserFromContext(ctx context.Context) *Session {
	session, _ := ctx.Value(UserKey).(*Session)
	return session
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
