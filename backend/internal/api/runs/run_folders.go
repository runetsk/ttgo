package runs

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// handleCreateRunFolder handles POST /run-folders
//
//	@Summary		Create a run folder
//	@Description	Create a new run folder with the given name, optionally nested under a parent
//	@Tags			run-folders
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{name=string,parent_id=string}	true	"Run folder name and optional parent_id"
//	@Success		201		{object}	models.RunFolder
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/run-folders [post]
func (h *Handler) CreateRunFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "name must be non-empty"})
		return
	}

	// Validate parent exists if provided
	if req.ParentID != nil && *req.ParentID != "" {
		parent, err := h.store.GetRunFolder(*req.ParentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to validate parent run folder", "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if parent == nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "parent folder not found"})
			return
		}
	}

	folder := &models.RunFolder{Name: strings.TrimSpace(req.Name), ParentID: req.ParentID}
	if err := h.store.CreateRunFolder(folder); err != nil {
		slog.ErrorContext(r.Context(), "failed to create run folder", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run folder created", "id", folder.ID, "name", folder.Name, "parent_id", folder.ParentID)
	httpx.JSON(w, http.StatusCreated, folder)
}

// handleGetRunFolders handles GET /run-folders
//
//	@Summary		List run folders
//	@Description	Return all run folders as a flat list (backward compat) or tree (?view=tree)
//	@Tags			run-folders
//	@Produce		json
//	@Param			view	query		string	false	"Set to 'tree' for hierarchical response"
//	@Success		200		{object}	map[string]interface{}	"run_folders array"
//	@Failure		500		{object}	map[string]string
//	@Router			/run-folders [get]
func (h *Handler) GetRunFolders(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")

	if view == "tree" {
		tree, err := h.store.GetRunFolderTree()
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to get run folder tree", "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if tree == nil {
			tree = []*models.RunFolder{}
		}
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"run_folders": tree})
		return
	}

	folders, err := h.store.GetRunFolders()
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get run folders", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"run_folders": folders})
}

// handleUpdateRunFolder handles PATCH /run-folders/{id}  (rename)
//
//	@Summary		Rename a run folder
//	@Description	Update the name of an existing run folder
//	@Tags			run-folders
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Run folder ID"
//	@Param			body	body		object{name=string}	true	"New name"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/run-folders/{id} [patch]
func (h *Handler) UpdateRunFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "name must be non-empty"})
		return
	}

	if err := h.store.UpdateRunFolder(id, strings.TrimSpace(req.Name)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "run folder not found"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to update run folder", "folder_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run folder renamed", "id", id, "new_name", req.Name)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleReorderRunFolder handles PATCH /run-folders/{id}/order
//
//	@Summary		Reorder a run folder
//	@Description	Update the display order of an existing run folder
//	@Tags			run-folders
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Run folder ID"
//	@Param			body	body		object{display_order=int}	true	"New display order"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/run-folders/{id}/order [patch]
func (h *Handler) ReorderRunFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		DisplayOrder *int `json:"display_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.DisplayOrder == nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "display_order is required"})
		return
	}

	if err := h.store.ReorderRunFolder(id, *req.DisplayOrder); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "run folder not found"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to reorder run folder", "folder_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run folder reordered", "id", id, "display_order", *req.DisplayOrder)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "reordered"})
}

// handleDeleteRunFolder handles DELETE /run-folders/{id}
//
//	@Summary		Delete a run folder
//	@Description	Delete an existing run folder and all its subfolders by ID
//	@Tags			run-folders
//	@Param			id	path	string	true	"Run folder ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/run-folders/{id} [delete]
func (h *Handler) DeleteRunFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteRunFolder(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "run folder not found"})
			return
		}
		slog.ErrorContext(r.Context(), "failed to delete run folder", "folder_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run folder deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// handleCopyRunFolder handles POST /run-folders/{id}/copy
//
//	@Summary		Copy a run folder
//	@Description	Deep-copy a run folder with all subfolders and runs (results reset to PENDING)
//	@Tags			run-folders
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string									true	"Source folder ID"
//	@Param			body	body		object{name=string,parent_id=string}	false	"Optional new name and parent"
//	@Success		201		{object}	models.RunFolder
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/run-folders/{id}/copy [post]
func (h *Handler) CopyRunFolder(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")

	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	folder, err := h.store.CopyRunFolder(sourceID, req.Name, req.ParentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to copy run folder", "source_id", sourceID, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run folder copied", "source_id", sourceID, "new_id", folder.ID)
	httpx.JSON(w, http.StatusCreated, folder)
}

// handleMoveRunFolder handles PATCH /run-folders/{id}/parent
//
//	@Summary		Move a run folder
//	@Description	Move a run folder to a new parent (or to root if parent_id is null)
//	@Tags			run-folders
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Run folder ID"
//	@Param			body	body		object{parent_id=string}	true	"New parent_id (null for root)"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/run-folders/{id}/parent [patch]
func (h *Handler) MoveRunFolder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	// Validate target parent exists if non-null
	if req.ParentID != nil && *req.ParentID != "" {
		parent, err := h.store.GetRunFolder(*req.ParentID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to validate parent run folder", "error", err)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if parent == nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "parent folder not found"})
			return
		}
	}

	if err := h.store.MoveRunFolder(id, req.ParentID); err != nil {
		if errors.Is(err, models.ErrCircularReference) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		slog.ErrorContext(r.Context(), "failed to move run folder", "folder_id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "run folder moved", "id", id, "new_parent", req.ParentID)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "moved"})
}
