package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/httputil"
)

type contextKey string

const (
	ContextUser contextKey = "user"
)

type ContextUserData struct {
	UserID  string
	IsAdmin bool
}

type Middleware struct {
	db *sql.DB
}

func NewMiddleware(db *sql.DB) *Middleware {
	return &Middleware{db: db}
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		var userID string
		var isAdmin bool
		var exp int64
		err := m.db.QueryRow(
			"SELECT user_id, is_admin, expires_at FROM sessions WHERE token=$1",
			token,
		).Scan(&userID, &isAdmin, &exp)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid session")
			return
		}

		if IsTokenExpired(exp) {
			m.db.Exec("DELETE FROM sessions WHERE token=$1", token)
			httputil.WriteError(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
			return
		}

		ctx := context.WithValue(r.Context(), ContextUser, ContextUserData{
			UserID:  userID,
			IsAdmin: isAdmin,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			httputil.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserFromContext(ctx context.Context) *ContextUserData {
	if v, ok := ctx.Value(ContextUser).(ContextUserData); ok {
		return &v
	}
	return nil
}

func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("session"); err == nil {
		return c.Value
	}
	return ""
}
