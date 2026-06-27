package authctx

import (
	"context"
	"net/http"
	"ttgo/pkg/tracker/models"
)

type contextKey string

const userKey contextKey = "authenticated_user"
const tokenKey contextKey = "authenticated_token"

// WithUser stores the authenticated user in the request context.
func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// FromContext extracts the authenticated user from a context.
func FromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userKey).(*models.User)
	return u
}

// WithToken stores the authenticating API token in the request context
// (Bearer-authenticated requests have no user, only a token).
func WithToken(ctx context.Context, token *models.ApiToken) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// TokenFromContext extracts the authenticating API token from a context.
func TokenFromContext(ctx context.Context) *models.ApiToken {
	t, _ := ctx.Value(tokenKey).(*models.ApiToken)
	return t
}

// ActorID returns a stable identifier for the authenticated principal: the user
// ID for session auth, "token:<id>" for Bearer-token auth, or "" if anonymous.
// This ensures token-driven writes are attributable rather than empty (F-024).
func ActorID(ctx context.Context) string {
	if u := FromContext(ctx); u != nil {
		return u.ID
	}
	if t := TokenFromContext(ctx); t != nil {
		return "token:" + t.ID
	}
	return ""
}

// UserFromRequest extracts the authenticated user from an HTTP request context.
func UserFromRequest(r *http.Request) *models.User {
	if r == nil {
		return nil
	}
	return FromContext(r.Context())
}
