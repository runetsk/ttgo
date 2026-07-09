package runs

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
)

// ListRunComments godoc
//
// @Summary      List comments on a run
// @Description  Returns all comments posted on the given test run.
// @Tags         runs
// @Produce      json
// @Param        id  path  string  true  "Test run ID"
// @Security     BearerAuth
// @Success      200  {array}   object{id=string,target_type=string,target_id=string,user_id=string,user_display_name=string,content=string,created_at=string,updated_at=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/comments [get]
func (h *Handler) ListRunComments(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	comments, err := h.store.ListComments("run", runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, enrichComments(comments))
}

// AddRunComment godoc
//
// @Summary      Add a comment to a run
// @Description  Posts a new comment (1-2000 characters) on the given test run.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id    path  string                 true  "Test run ID"
// @Param        body  body  object{content=string}  true  "Comment text"
// @Security     BearerAuth
// @Success      201  {object}  object{id=string,target_type=string,target_id=string,user_id=string,user_display_name=string,content=string,created_at=string,updated_at=string}
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/comments [post]
func (h *Handler) AddRunComment(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	run, err := h.store.GetTestRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if run == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if len(content) == 0 || len(content) > 2000 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content must be 1-2000 characters"})
		return
	}

	userID := authctx.ActorID(r.Context())

	comment := &models.Comment{
		TargetType: "run",
		TargetID:   runID,
		UserID:     userID,
		Content:    content,
	}
	if err := h.store.CreateComment(comment); err != nil {
		slog.ErrorContext(r.Context(), "failed to create run comment", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if fullRun, err := h.store.GetTestRun(runID); err == nil && fullRun != nil && h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventCommentAdded, "run:"+runID, fullRun))
	}

	httpx.JSON(w, http.StatusCreated, enrichComment(*comment))
}

// ListResultComments godoc
//
// @Summary      List comments on a run result
// @Description  Returns all comments posted on the given run result.
// @Tags         runs
// @Produce      json
// @Param        id         path  string  true  "Test run ID"
// @Param        result_id  path  string  true  "Run result ID"
// @Security     BearerAuth
// @Success      200  {array}   object{id=string,target_type=string,target_id=string,user_id=string,user_display_name=string,content=string,created_at=string,updated_at=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/comments [get]
func (h *Handler) ListResultComments(w http.ResponseWriter, r *http.Request) {
	resultID := r.PathValue("result_id")

	comments, err := h.store.ListComments("result", resultID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, enrichComments(comments))
}

// AddResultComment godoc
//
// @Summary      Add a comment to a run result
// @Description  Posts a new comment (1-2000 characters) on the given run result.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id         path  string                  true  "Test run ID"
// @Param        result_id  path  string                  true  "Run result ID"
// @Param        body       body  object{content=string}  true  "Comment text"
// @Security     BearerAuth
// @Success      201  {object}  object{id=string,target_type=string,target_id=string,user_id=string,user_display_name=string,content=string,created_at=string,updated_at=string}
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/comments [post]
func (h *Handler) AddResultComment(w http.ResponseWriter, r *http.Request) {
	resultID := r.PathValue("result_id")
	if !h.store.RunResultExists(resultID) {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if len(content) == 0 || len(content) > 2000 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content must be 1-2000 characters"})
		return
	}

	userID := authctx.ActorID(r.Context())

	comment := &models.Comment{
		TargetType: "result",
		TargetID:   resultID,
		UserID:     userID,
		Content:    content,
	}
	if err := h.store.CreateComment(comment); err != nil {
		slog.ErrorContext(r.Context(), "failed to create result comment", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, enrichComment(*comment))
}

// UpdateComment godoc
//
// @Summary      Update a comment
// @Description  Edits the text of an existing comment (1-2000 characters). Only the comment's author or an admin may update it.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        comment_id  path  string                  true  "Comment ID"
// @Param        body        body  object{content=string}  true  "New comment text"
// @Security     BearerAuth
// @Success      200  {object}  object{id=string,target_type=string,target_id=string,user_id=string,user_display_name=string,content=string,created_at=string,updated_at=string}
// @Failure      400  {object}  object{error=string}
// @Failure      403  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /comments/{comment_id} [put]
func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("comment_id")
	user := authctx.UserFromRequest(r)

	existing, err := h.store.GetComment(commentID)
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "comment not found"})
		return
	}
	if user == nil || (user.Role != "admin" && existing.UserID != user.ID) {
		httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "not authorized"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if len(content) == 0 || len(content) > 2000 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content must be 1-2000 characters"})
		return
	}

	updated, err := h.store.UpdateComment(commentID, content)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, enrichComment(*updated))
}

// DeleteComment godoc
//
// @Summary      Delete a comment
// @Description  Deletes an existing comment. Only the comment's author or an admin may delete it.
// @Tags         runs
// @Produce      json
// @Param        comment_id  path  string  true  "Comment ID"
// @Security     BearerAuth
// @Success      200  {object}  object{status=string}
// @Failure      403  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /comments/{comment_id} [delete]
func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("comment_id")
	user := authctx.UserFromRequest(r)

	existing, err := h.store.GetComment(commentID)
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "comment not found"})
		return
	}
	if user == nil || (user.Role != "admin" && existing.UserID != user.ID) {
		httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "not authorized"})
		return
	}

	targetType := existing.TargetType
	targetID := existing.TargetID

	if err := h.store.DeleteComment(commentID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if targetType == "run" {
		if fullRun, err := h.store.GetTestRun(targetID); err == nil && fullRun != nil && h.hub != nil {
			h.hub.Broadcast(apiws.NewEvent(apiws.EventCommentDeleted, "run:"+targetID, fullRun))
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type commentResponse struct {
	ID              string `json:"id"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	UserID          string `json:"user_id"`
	UserDisplayName string `json:"user_display_name"`
	Content         string `json:"content"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func enrichComment(c models.Comment) commentResponse {
	displayName := "API"
	if c.User != nil {
		displayName = c.User.DisplayName
	}
	return commentResponse{
		ID:              c.ID,
		TargetType:      c.TargetType,
		TargetID:        c.TargetID,
		UserID:          c.UserID,
		UserDisplayName: displayName,
		Content:         c.Content,
		CreatedAt:       c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       c.UpdatedAt.Format(time.RFC3339),
	}
}

func enrichComments(comments []models.Comment) []commentResponse {
	result := make([]commentResponse, len(comments))
	for i, c := range comments {
		result[i] = enrichComment(c)
	}
	return result
}
