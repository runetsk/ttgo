package users

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"golang.org/x/crypto/bcrypt"
)

type UserFromRequestFunc func(*http.Request) *models.User
type LogAuthEventFunc func(userID, action, diff string)

type Handler struct {
	store           *store.Store
	hub             *apiws.Hub
	userFromRequest UserFromRequestFunc
	logAuthEvent    LogAuthEventFunc
}

func NewHandler(s *store.Store, hub *apiws.Hub, userFromRequest UserFromRequestFunc, logAuthEvent LogAuthEventFunc) *Handler {
	return &Handler{
		store:           s,
		hub:             hub,
		userFromRequest: userFromRequest,
		logAuthEvent:    logAuthEvent,
	}
}

// handleListUsers processes GET /api/users (admin only).
//
// @Summary      List users
// @Description  Returns all user accounts. Admin session required.
// @Tags         users
// @Security     SessionCookie
// @Produce      json
// @Success      200  {object}  object{users=array,total=integer}
// @Failure      403  {object}  object{error=string}
// @Router       /users [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	includeDeleted := r.URL.Query().Get("include_deleted") == "true"
	list, err := h.store.ListUsers(includeDeleted)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list users"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"users": list,
		"total": len(list),
	})
}

// handleCreateUser processes POST /api/users (admin only).
//
// @Summary      Create user
// @Description  Create a new user account. Admin session required.
// @Tags         users
// @Security     SessionCookie
// @Accept       json
// @Produce      json
// @Param        body  body  object{email=string,display_name=string,password=string,role=string}  true  "User data"
// @Success      201  {object}  object
// @Failure      400  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Router       /users [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var admin *models.User
	if h.userFromRequest != nil {
		admin = h.userFromRequest(r)
	}

	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(req.Email)); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "email is not a valid address"})
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "password must be 8-72 characters"})
		return
	}
	role := req.Role
	if role == "" {
		role = "member"
	}
	if role != "admin" && role != "member" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "role must be 'admin' or 'member'"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	user, err := h.store.CreateUser(req.Email, req.DisplayName, string(hash), role)
	if err != nil {
		// Detect unique constraint violation
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "email already in use"})
			return
		}
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create user"})
		return
	}

	creatorID := ""
	if admin != nil {
		creatorID = admin.ID
	}
	if h.logAuthEvent != nil {
		h.logAuthEvent(creatorID, "user.created", "email="+user.Email)
	}

	// 018-websocket-realtime: broadcast user created
	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventUserUpdated, "settings:*", user))
	}

	httpx.JSON(w, http.StatusCreated, user)
}

// handleUpdateUser processes PATCH /api/users/{id} (admin only).
//
// @Summary      Update user
// @Description  Partially update a user account (deactivate, reactivate, change role, reset password). Admin session required.
// @Tags         users
// @Security     SessionCookie
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "User ID"
// @Param        body  body  object{display_name=string,active=boolean,role=string,password=string}  false  "Fields to update (hidden removed)"
// @Success      200  {object}  object
// @Failure      400  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Router       /users/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var admin *models.User
	if h.userFromRequest != nil {
		admin = h.userFromRequest(r)
	}
	id := r.PathValue("id")

	target, err := h.store.GetUser(id)
	if err != nil || target == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	var req struct {
		DisplayName *string `json:"display_name"`
		Active      *bool   `json:"active"`
		Role        *string `json:"role"`
		Password    *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	updates := map[string]interface{}{}

	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Role != nil {
		if *req.Role != "admin" && *req.Role != "member" {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "role must be 'admin' or 'member'"})
			return
		}
		// Guard: cannot demote the last active admin to member, which would leave
		// the system with no admin (mirrors the deactivate guard below) (F-051).
		if *req.Role == "member" && target.Role == "admin" && target.Active {
			count, err := h.store.CountActiveAdmins()
			if err != nil {
				httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			if count <= 1 {
				httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "cannot demote the last active admin"})
				return
			}
		}
		updates["role"] = *req.Role
	}
	if req.Active != nil {
		if !*req.Active && target.Role == "admin" {
			// Guard: cannot deactivate last admin
			count, err := h.store.CountActiveAdmins()
			if err != nil {
				httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			if count <= 1 {
				httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "cannot deactivate the last active admin"})
				return
			}
		}
		updates["active"] = *req.Active

		// If deactivating, evict live sessions
		if !*req.Active {
			_ = h.store.DeleteUserSessions(id) //nolint:errcheck
		}
	}
	if req.Password != nil {
		if len(*req.Password) < 8 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), 12)
		if err != nil {
			httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		updates["hashed_password"] = string(hash)
	}

	updated, err := h.store.UpdateUser(id, updates)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update user"})
		return
	}

	adminID := ""
	if admin != nil {
		adminID = admin.ID
	}

	// Determine audit action
	if req.Active != nil && !*req.Active {
		if h.logAuthEvent != nil {
			h.logAuthEvent(adminID, "user.deactivated", "target="+id)
		}
	} else if req.Active != nil && *req.Active {
		if h.logAuthEvent != nil {
			h.logAuthEvent(adminID, "user.reactivated", "target="+id)
		}
	}
	if req.Password != nil {
		if h.logAuthEvent != nil {
			h.logAuthEvent(adminID, "user.password_reset", "target="+id)
		}
	}

	// 018-websocket-realtime: broadcast user updated
	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventUserUpdated, "settings:*", updated))
	}

	httpx.JSON(w, http.StatusOK, updated)
}

// handleDeleteUser processes DELETE /api/users/{id} (admin only).
//
// @Summary      Soft-delete user
// @Description  Marks a user as deleted. Evicts all sessions. Admin session required.
// @Tags         users
// @Security     SessionCookie
// @Param        id  path  string  true  "User ID"
// @Success      200  {object}  object
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Router       /users/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	var admin *models.User
	if h.userFromRequest != nil {
		admin = h.userFromRequest(r)
	}
	id := r.PathValue("id")

	// Guard: cannot delete yourself
	if admin != nil && admin.ID == id {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete yourself"})
		return
	}

	target, err := h.store.GetUser(id)
	if err != nil || target == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	if target.Deleted {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "user is already deleted"})
		return
	}

	// Guard: cannot delete last active admin
	if target.Role == "admin" && target.Active {
		count, err := h.store.CountActiveAdmins()
		if err != nil {
			httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if count <= 1 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete the last active admin"})
			return
		}
	}

	updated, err := h.store.UpdateUser(id, map[string]interface{}{
		"deleted": true,
		"active":  false,
	})
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete user"})
		return
	}

	_ = h.store.DeleteUserSessions(id)

	adminID := ""
	if admin != nil {
		adminID = admin.ID
	}
	if h.logAuthEvent != nil {
		h.logAuthEvent(adminID, "user.deleted", "target="+id)
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventUserUpdated, "settings:*", updated))
	}

	httpx.JSON(w, http.StatusOK, updated)
}

// handleRestoreUser processes POST /api/users/{id}/restore (admin only).
//
// @Summary      Restore deleted user
// @Description  Restores a soft-deleted user. User comes back as inactive. Admin session required.
// @Tags         users
// @Security     SessionCookie
// @Param        id  path  string  true  "User ID"
// @Success      200  {object}  object
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Router       /users/{id}/restore [post]
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	var admin *models.User
	if h.userFromRequest != nil {
		admin = h.userFromRequest(r)
	}
	id := r.PathValue("id")

	target, err := h.store.GetUser(id)
	if err != nil || target == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	if !target.Deleted {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "user is not deleted"})
		return
	}

	updated, err := h.store.UpdateUser(id, map[string]interface{}{
		"deleted": false,
	})
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "could not restore user"})
		return
	}

	adminID := ""
	if admin != nil {
		adminID = admin.ID
	}
	if h.logAuthEvent != nil {
		h.logAuthEvent(adminID, "user.restored", "target="+id)
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventUserUpdated, "settings:*", updated))
	}

	httpx.JSON(w, http.StatusOK, updated)
}
