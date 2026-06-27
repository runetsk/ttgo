package search

import (
	"log/slog"
	"net/http"
	"strconv"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

// handleSearch godoc
// @Summary      Search test cases
// @Description  Full-text search across test cases
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        q       query     string  true   "Search query"
// @Param        limit   query     int     false  "Maximum number of results to return"  default(50)
// @Param        offset  query     int     false  "Number of results to skip"            default(0)
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]string
// @Router       /search [get]
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = httpx.ClampLimit(v, 50, 200) // cap to bound result-set memory (F-044)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	if q == "" {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{
			"results": []interface{}{},
			"total":   0,
			"query":   q,
		})
		return
	}

	results, total, err := h.store.SearchTestCases(q, limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "search failed", "query", q, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   total,
		"query":   q,
	})
}
