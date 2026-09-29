package search

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/store"
)

// Reranker re-orders the first page of results (TypeSafe.ai, spec Wave 5 §4) and reports whether
// it did; on any problem it returns the results unchanged and false.
type Reranker func(ctx context.Context, query string, results []store.SearchResult) ([]store.SearchResult, bool)

type Handler struct {
	store  *store.Store
	rerank Reranker
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

// SetReranker wires the re-ranker used for ?rerank=true (nil = never re-rank).
func (h *Handler) SetReranker(r Reranker) { h.rerank = r }

// handleSearch godoc
// @Summary      Search test cases
// @Description  Full-text search across test cases
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        q       query     string  true   "Search query"
// @Param        limit   query     int     false  "Maximum number of results to return"  default(50)
// @Param        offset  query     int     false  "Number of results to skip"            default(0)
// @Param        rerank  query     bool    false  "Re-rank the first page with TypeSafe.ai when its search re-ranking is on (results then carry relevance; the response says reranked)"
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

	// The first page may be re-ranked by TypeSafe.ai; any problem keeps the BM25 order.
	reranked := false
	if want, _ := strconv.ParseBool(r.URL.Query().Get("rerank")); want && offset == 0 && h.rerank != nil && len(results) > 1 {
		results, reranked = h.rerank(r.Context(), q, results)
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"results":  results,
		"total":    total,
		"query":    q,
		"reranked": reranked,
	})
}
