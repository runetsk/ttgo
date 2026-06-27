package store

import (
	"fmt"
	"strings"
	"time"
)

// analyticsQuery builds parameterized SQL queries for analytics methods.
type analyticsQuery struct {
	base    string
	joins   []string
	wheres  []string
	args    []interface{}
	groupBy string
	orderBy string
	limit   int
}

func newAnalyticsQuery(base string) *analyticsQuery {
	return &analyticsQuery{base: base}
}

func (q *analyticsQuery) Join(clause string) *analyticsQuery {
	q.joins = append(q.joins, clause)
	return q
}

func (q *analyticsQuery) Where(clause string, args ...interface{}) *analyticsQuery {
	q.wheres = append(q.wheres, clause)
	q.args = append(q.args, args...)
	return q
}

func (q *analyticsQuery) GroupBy(clause string) *analyticsQuery {
	q.groupBy = clause
	return q
}

func (q *analyticsQuery) OrderBy(clause string) *analyticsQuery {
	q.orderBy = clause
	return q
}

func (q *analyticsQuery) Limit(n int) *analyticsQuery {
	q.limit = n
	return q
}

func (q *analyticsQuery) Build() (string, []interface{}) {
	var sb strings.Builder
	sb.WriteString(q.base)
	for _, j := range q.joins {
		sb.WriteString(" ")
		sb.WriteString(j)
	}
	if len(q.wheres) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(q.wheres, " AND "))
	}
	if q.groupBy != "" {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(q.groupBy)
	}
	if q.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(q.orderBy)
	}
	if q.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
	}
	return sb.String(), q.args
}

// applyRunResultFilters adds common date range (on start_time) and folder_id filters
// for queries on run_results. The alias parameter is the table alias for the
// run_results table (e.g. "rr"). When folderID is non-empty a JOIN to test_runs is
// added using a derived alias ("<alias>_tr") and a run_folder_id filter is applied.
func (q *analyticsQuery) applyRunResultFilters(alias string, folderID string, startDate, endDate time.Time) *analyticsQuery {
	if folderID != "" {
		trAlias := alias + "_tr"
		q.Join(fmt.Sprintf("JOIN test_runs %s ON %s.test_run_id = %s.id", trAlias, alias, trAlias))
	}
	if !startDate.IsZero() {
		q.Where(fmt.Sprintf("%s.start_time >= ?", alias), startDate)
	}
	if !endDate.IsZero() {
		q.Where(fmt.Sprintf("%s.start_time < ?", alias), endDate)
	}
	if folderID != "" {
		trAlias := alias + "_tr"
		q.Where(fmt.Sprintf("%s.run_folder_id = ?", trAlias), folderID)
	}
	return q
}
