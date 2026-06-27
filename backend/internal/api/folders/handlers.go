package folders

import (
	"encoding/json"
	"errors"
	"net/http"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"gorm.io/gorm"
)

type Handler struct {
	store *store.Store
	hub   *apiws.Hub
}

func NewHandler(s *store.Store, hub *apiws.Hub) *Handler {
	return &Handler{store: s, hub: hub}
}

// handleGetFolderTree returns the full folder hierarchy.
//
// @Summary      Get folder tree
// @Description  Returns the complete folder hierarchy with nested sub-folders and test cases.
// @Tags         folders
// @Produce      json
// @Success      200  {array}   models.Folder
// @Failure      500  {object}  map[string]string
// @Router       /folders/tree [get]
func (h *Handler) GetTree(w http.ResponseWriter, r *http.Request) {
	tree, err := h.store.GetFolderTree()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tree)
}

// handleGetFolder returns a single folder by ID.
//
// @Summary      Get folder by ID
// @Description  Returns a folder with its metadata.
// @Tags         folders
// @Produce      json
// @Param        id   path      string  true  "Folder UUID"
// @Success      200  {object}  models.Folder
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /folders/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	folder, err := h.store.GetFolder(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
		} else {
			httpx.Error(w, http.StatusInternalServerError, err)
		}
		return
	}
	httpx.JSON(w, http.StatusOK, folder)
}

// handleCreateFolder creates a new folder.
//
// @Summary      Create folder
// @Description  Creates a new folder, optionally nested under a parent folder.
// @Tags         folders
// @Accept       json
// @Produce      json
// @Param        body  body      object{name=string,parent_id=string}  true  "Folder data"
// @Success      201   {object}  models.Folder
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /folders [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if len(req.Name) > 255 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "name must be at most 255 characters"})
		return
	}

	if req.ParentID != nil && *req.ParentID != "" {
		if _, err := h.store.GetFolder(*req.ParentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "parent folder not found"})
			} else {
				httpx.Error(w, http.StatusInternalServerError, err)
			}
			return
		}
	}

	folder, err := h.store.CreateFolder(req.Name, req.ParentID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventFolderCreated, "folders:*", folder))
	}

	httpx.JSON(w, http.StatusCreated, folder)
}

// handleRenameFolder renames a folder by ID.
//
// @Summary      Rename folder
// @Description  Updates the name of a folder.
// @Tags         folders
// @Accept       json
// @Produce      json
// @Param        id    path      string              true  "Folder UUID"
// @Param        body  body      object{name=string} true  "New folder name"
// @Success      200   {object}  object{id=string,name=string}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /folders/{id} [patch]
func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	if err := h.store.RenameFolder(id, req.Name); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	folder, err := h.store.GetFolder(id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventFolderUpdated, "folders:*", folder))
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"id":   folder.ID,
		"name": folder.Name,
	})
}

// handleDeleteFolder deletes a folder by ID.
//
// @Summary      Delete folder
// @Description  Deletes a folder by ID.
// @Tags         folders
// @Param        id   path  string  true  "Folder UUID"
// @Success      204  "Deleted"
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /folders/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if err := h.store.DeleteFolder(id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventFolderDeleted, "folders:*", map[string]string{"id": id}))
	}

	httpx.JSON(w, http.StatusNoContent, nil)
}

// handleBulkDeleteFolders deletes multiple folders by IDs.
//
// @Summary      Bulk delete folders
// @Description  Deletes multiple folders by their IDs.
// @Tags         folders
// @Accept       json
// @Param        body  body  object{ids=[]string}  true  "List of folder UUIDs"
// @Success      204   "Deleted"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /folders/bulk-delete [post]
func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if len(req.IDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "ids required"})
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}

	if err := h.store.BulkDeleteFolders(req.IDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventFolderDeleted, "folders:*", map[string]interface{}{"ids": req.IDs}))
	}

	httpx.JSON(w, http.StatusNoContent, nil)
}

// handleMoveFolder moves a folder to a new parent.
//
// @Summary      Move folder
// @Description  Updates the parent of a folder. Pass null parent_id to move to root.
// @Tags         folders
// @Accept       json
// @Param        id    path  string                    true  "Folder UUID"
// @Param        body  body  object{parent_id=string}  true  "New parent ID (or null for root)"
// @Success      204   "Moved"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /folders/{id}/parent [patch]
func (h *Handler) Move(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	var req struct {
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.ParentID != nil && *req.ParentID != "" {
		if _, err := h.store.GetFolder(*req.ParentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "parent folder not found"})
			} else {
				httpx.Error(w, http.StatusInternalServerError, err)
			}
			return
		}
	}

	if err := h.store.MoveFolder(id, req.ParentID); err != nil {
		if errors.Is(err, models.ErrCircularReference) {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventFolderUpdated, "folders:*", map[string]string{"id": id}))
	}

	httpx.JSON(w, http.StatusNoContent, nil)
}

// handleBulkMoveFolders moves multiple folders to a new parent.
//
// @Summary      Bulk move folders
// @Description  Moves multiple folders to a new parent folder.
// @Tags         folders
// @Accept       json
// @Param        body  body  object{ids=[]string,parent_id=string}  true  "Folder IDs and destination parent"
// @Success      204   "Moved"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /folders/bulk-move [post]
func (h *Handler) BulkMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs      []string `json:"ids"`
		ParentID *string  `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if len(req.IDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "ids required"})
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}

	if req.ParentID != nil && *req.ParentID != "" {
		if _, err := h.store.GetFolder(*req.ParentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "parent folder not found"})
			} else {
				httpx.Error(w, http.StatusInternalServerError, err)
			}
			return
		}
	}

	if err := h.store.BulkMoveFolders(req.IDs, req.ParentID); err != nil {
		if errors.Is(err, models.ErrCircularReference) {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventFolderUpdated, "folders:*", map[string]interface{}{"ids": req.IDs}))
	}

	httpx.JSON(w, http.StatusNoContent, nil)
}
