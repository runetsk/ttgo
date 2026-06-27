package auth

import (
	"encoding/json"
	"net/http"
	"time"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserFromRequestFunc func(*http.Request) *models.User

// Handler serves auth endpoints.
type Handler struct {
	store           *store.Store
	userFromRequest UserFromRequestFunc
}

func NewHandler(s *store.Store, userFromRequest UserFromRequestFunc) *Handler {
	return &Handler{store: s, userFromRequest: userFromRequest}
}

// logAuthEvent persists an AuditLog entry for auth-related events.
// Uses the public store method (CreateAuditLog) rather than accessing db directly.
func (h *Handler) logAuthEvent(userID, action, diff string) {
	entry := &models.AuditLog{
		ID:         uuid.New().String(),
		TestCaseID: "",
		UserID:     userID,
		Action:     action,
		Diff:       diff,
		Timestamp:  time.Now(),
	}
	_ = h.store.CreateAuditLog(entry) //nolint:errcheck
}

func (h *Handler) LogAuthEvent(userID, action, diff string) {
	h.logAuthEvent(userID, action, diff)
}

// handleLogin processes POST /api/auth/login.
//
// @Summary      Login
// @Description  Authenticate with email and password; sets a session_token cookie on success.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object{email=string,password=string}  true  "Credentials"
// @Success      200  {object}  object{user=object}
// @Failure      401  {object}  object{error=string}
// @Router       /auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, err := h.store.FindUserByEmail(req.Email)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if user == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	// Verify password BEFORE revealing account state, so an attacker without the
	// correct password cannot tell a disabled account from a wrong password (F-042).
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		h.logAuthEvent(user.ID, "auth.login.failed", "")
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid email or password"})
		return
	}

	// Only after a correct password do we reveal that the account is disabled.
	if !user.Active {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "account is disabled"})
		return
	}

	// Successful login
	_ = h.store.DeleteExpiredSessions() //nolint:errcheck

	session, err := h.store.CreateSession(user.ID)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	h.logAuthEvent(user.ID, "auth.login", "")

	httpx.JSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

// handleLogout processes POST /api/auth/logout.
//
// @Summary      Logout
// @Description  Invalidate the current session and clear the session cookie.
// @Tags         auth
// @Security     SessionCookie
// @Success      204
// @Router       /auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		_ = h.store.DeleteSession(cookie.Value) //nolint:errcheck
	}

	// Expire the cookie regardless
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	// Log if user context is available
	if h.userFromRequest != nil {
		if u := h.userFromRequest(r); u != nil {
			h.logAuthEvent(u.ID, "auth.logout", "")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleMe processes GET /api/auth/me.
//
// @Summary      Current user
// @Description  Returns the currently authenticated user from the session cookie.
// @Tags         auth
// @Security     SessionCookie
// @Produce      json
// @Success      200  {object}  object{user=object}
// @Failure      401  {object}  object{error=string}
// @Router       /auth/me [get]
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	user, err := h.store.ValidateSession(cookie.Value)
	if err != nil || user == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

// handleChangePassword processes POST /api/auth/change-password.
// Requires session auth; the user is taken from the request context.
//
// @Summary      Change password
// @Description  Change the currently authenticated user's password. Requires session cookie.
// @Tags         auth
// @Security     SessionCookie
// @Accept       json
// @Param        body  body  object{current_password=string,new_password=string}  true  "Passwords"
// @Success      204
// @Failure      400  {object}  object{error=string}
// @Failure      401  {object}  object{error=string}
// @Router       /auth/change-password [post]
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var u *models.User
	if h.userFromRequest != nil {
		u = h.userFromRequest(r)
	}
	if u == nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "session required"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.NewPassword) < 8 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 8 characters"})
		return
	}
	// bcrypt silently truncates at 72 bytes, so reject longer inputs rather than
	// give a false sense of strength; and require an actual change (F-049).
	if len(req.NewPassword) > 72 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at most 72 bytes"})
		return
	}
	if req.NewPassword == req.CurrentPassword {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "new password must differ from the current password"})
		return
	}

	// Re-fetch user to get HashedPassword (context user may lack it)
	full, err := h.store.GetUser(u.ID)
	if err != nil || full == nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(full.HashedPassword), []byte(req.CurrentPassword)); err != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if _, err := h.store.UpdateUser(u.ID, map[string]interface{}{"hashed_password": string(hash)}); err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update password"})
		return
	}

	// Invalidate all existing sessions and create a fresh one
	_ = h.store.DeleteUserSessions(u.ID) //nolint:errcheck

	session, err := h.store.CreateSession(u.ID)
	if err != nil {
		// Non-fatal: password changed; just clear the cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    session.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
		})
	}

	h.logAuthEvent(u.ID, "user.password_changed", "")

	w.WriteHeader(http.StatusNoContent)
}
