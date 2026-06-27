package api

import (
	"encoding/json"
	"net/http"
	"time"
	"ttgo/internal/api/httpx"

	"log/slog"
)

// handleGetSeedStatus returns the current demo-data seed status.
//
// @Summary      Get seed status
// @Description  Return whether demo data is currently seeded and summary counts.
// @Tags         seed
// @Produce      json
// @Success      200  {object}  object
// @Failure      500  {object}  map[string]string
// @Router       /seed [get]
// @Security     BearerAuth
func (s *Server) handleGetSeedStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.store.GetSeedStatus()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: GetSeedStatus failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

// handleCreateSeed loads the demo dataset, optionally replacing existing demo data.
//
// @Summary      Seed demo data
// @Description  Load the demo dataset into the database. Replaces existing demo data if present. Admin only.
// @Tags         seed
// @Produce      json
// @Success      201  {object}  object
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /seed [post]
// @Security     BearerAuth
func (s *Server) handleCreateSeed(w http.ResponseWriter, r *http.Request) {
	if !s.seedMu.TryLock() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "seed operation already in progress"})
		return
	}
	defer s.seedMu.Unlock()

	user := userFromContext(r)
	userID := ""
	if user != nil {
		userID = user.ID
	}

	start := time.Now()
	slog.InfoContext(r.Context(), "seed: operation started", "user_id", userID)

	existing, err := s.store.GetSeedStatus()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: GetSeedStatus failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	result, err := s.store.SeedDemoTx(existing.HasDemoData)
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: operation failed", "duration", time.Since(start), "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "seed: completed",
		"duration", time.Since(start),
		"folders", result.Created.Folders,
		"categories", result.Created.Categories,
		"test_cases", result.Created.TestCases,
		"runs", result.Created.TestRuns,
		"results", result.Created.RunResults,
		"replaced", result.ReplacedExisting,
	)

	httpx.JSON(w, http.StatusCreated, result)
}

// handleDeleteSeed removes all demo-seeded entities.
//
// @Summary      Remove demo data
// @Description  Remove all demo-seeded entities from the database. Returns 204 if no demo data exists.
// @Tags         seed
// @Produce      json
// @Success      200  {object}  object
// @Success      204
// @Failure      500  {object}  map[string]string
// @Router       /seed [delete]
// @Security     BearerAuth
func (s *Server) handleDeleteSeed(w http.ResponseWriter, r *http.Request) {
	// Serialize with create/reset so concurrent destructive seed ops cannot
	// interleave their transactions and corrupt the demo dataset (F-038).
	if !s.seedMu.TryLock() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "seed operation already in progress"})
		return
	}
	defer s.seedMu.Unlock()

	existing, err := s.store.GetSeedStatus()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: GetSeedStatus failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if !existing.HasDemoData {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	user := userFromContext(r)
	userID := ""
	if user != nil {
		userID = user.ID
	}

	start := time.Now()
	slog.InfoContext(r.Context(), "seed: remove started", "user_id", userID)

	deleteResult, err := s.store.RemoveSeedTx()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: remove failed", "duration", time.Since(start), "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.InfoContext(r.Context(), "seed: remove completed",
		"duration", time.Since(start),
		"folders", deleteResult.Deleted.Folders,
		"categories", deleteResult.Deleted.Categories,
		"test_cases", deleteResult.Deleted.TestCases,
		"runs", deleteResult.Deleted.TestRuns,
		"results", deleteResult.Deleted.RunResults,
	)

	httpx.JSON(w, http.StatusOK, deleteResult)
}

// handleResetAllData erases ALL application data (preserving user accounts).
//
// @Summary      Reset all data
// @Description  Erase ALL application data (test cases, runs, folders, etc.) while preserving user accounts. Requires confirmation body {"confirm": "CONFIRM RESET"}. Admin only.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body  object{confirm=string}  true  "Confirmation payload"
// @Success      200  {object}  object
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /admin/reset [delete]
// @Security     BearerAuth
func (s *Server) handleResetAllData(w http.ResponseWriter, r *http.Request) {
	var confirmReq struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&confirmReq); err != nil || confirmReq.Confirm != "CONFIRM RESET" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "must send {\"confirm\": \"CONFIRM RESET\"} to proceed"})
		return
	}

	if !s.seedMu.TryLock() {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "data operation already in progress"})
		return
	}
	defer s.seedMu.Unlock()

	user := userFromContext(r)
	userID := ""
	if user != nil {
		userID = user.ID
	}

	start := time.Now()
	slog.WarnContext(r.Context(), "admin/reset: FULL DATA RESET initiated", "user_id", userID)

	counts, err := s.store.ResetAllDataTx()
	if err != nil {
		slog.ErrorContext(r.Context(), "admin/reset: failed", "duration", time.Since(start), "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	slog.WarnContext(r.Context(), "admin/reset: completed, all data erased", "duration", time.Since(start))
	httpx.JSON(w, http.StatusOK, counts)
}
