package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/store"

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

// handleGetAISeedStatus reports the AI demo dataset's state and its answer key.
// @Summary      Get AI demo dataset status and answer key
// @Description  Reports whether the AI failure-analysis demo dataset is loaded, the deterministic id of its newest run, and the planted templates' expected verdict and defect type. The answer key is static, so it is available before seeding; `ttgo ai compare` uses it to grade analyses.
// @Tags         seed
// @Produce      json
// @Success      200  {object}  object{loaded=bool,latest_run_id=string,ground_truth=[]store.AISeedGroundTruth}
// @Failure      500  {object}  object{error=string}
// @Security     BearerAuth
// @Router       /seed/ai [get]
func (s *Server) handleGetAISeedStatus(w http.ResponseWriter, r *http.Request) {
	loaded, err := s.store.HasAIDemoData()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: HasAIDemoData failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	gt, err := store.AIDemoGroundTruth()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: AIDemoGroundTruth failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"loaded":        loaded,
		"latest_run_id": store.AIDemoLatestRunID(),
		"ground_truth":  gt,
	})
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

// handleCreateAISeed loads the AI failure-analysis demo dataset.
//
// @Summary      Seed AI failure-analysis demo data
// @Description  Load the AI failure-analysis demo dataset (30 daily runs x 500 results with planted failure groups, triage history, and a ground-truth answer key). Replaces a previously loaded AI demo dataset. Admin only.
// @Tags         seed
// @Produce      json
// @Success      201  {object}  object
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Accept       json
// @Param        body  body  object{failure_scale=int}  false  "Optional: multiply the planted failures (1-5); omit for the default dataset"
// @Router       /seed/ai [post]
// @Security     BearerAuth
func (s *Server) handleCreateAISeed(w http.ResponseWriter, r *http.Request) {
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

	scale, err := aiSeedScale(r)
	if err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	start := time.Now()
	slog.InfoContext(r.Context(), "seed: ai demo operation started", "user_id", userID, "failure_scale", scale)

	replaced, err := s.store.HasAIDemoData()
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: HasAIDemoData failed", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	result, err := s.store.SeedAIDemoTxWithScale(scale)
	if err != nil {
		slog.ErrorContext(r.Context(), "seed: ai demo operation failed", "duration", time.Since(start), "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	result.ReplacedExisting = replaced

	slog.InfoContext(r.Context(), "seed: ai demo completed",
		"duration", time.Since(start),
		"runs", result.Created.TestRuns,
		"results", result.Created.RunResults,
		"failing", result.FailingRows,
		"labeled", result.LabeledRows,
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

// aiSeedScale reads the optional {"failure_scale": n} body of POST /seed/ai.
// No body, an empty body, or a body without the field means the default
// dataset (scale 1).
func aiSeedScale(r *http.Request) (int, error) {
	if r.Body == nil || r.ContentLength == 0 {
		return 1, nil
	}
	var req struct {
		FailureScale *int `json:"failure_scale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return 1, nil
		}
		return 0, fmt.Errorf("invalid body: %w", err)
	}
	if req.FailureScale == nil {
		return 1, nil
	}
	if *req.FailureScale < 1 || *req.FailureScale > 5 {
		return 0, fmt.Errorf("failure_scale must be between 1 and 5")
	}
	return *req.FailureScale, nil
}
