package httpx

// MaxBulkIDs is the maximum number of IDs accepted by a single bulk operation
// (bulk-delete, bulk-update, export). Requests above this are rejected with 400
// to prevent unbounded mass mutations and oversized SQL IN(...) clauses.
const MaxBulkIDs = 500

// ClampLimit normalises a caller-supplied pagination limit: non-positive values
// fall back to def, values above max are capped at max.
func ClampLimit(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// ClampOffset returns a non-negative offset.
func ClampOffset(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
