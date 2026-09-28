// Package compare pivots a run's AI failure analyses by engine and model so the
// verdicts of TypeSafe and of generative LLMs can be read side by side, graded
// against the AI demo dataset's answer key, and summarized. It is pure: the CLI
// command fetches the JSON and hands it over.
package compare

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"ttgo/pkg/tracker/models"
)

// Result is one failing run result.
type Result struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}

// Analysis is one stored analysis version (the wire shape of
// GET /api/run-results/{id}/analyses, without the raw response).
type Analysis struct {
	RunResultID                   string   `json:"run_result_id"`
	Version                       int      `json:"version"`
	Engine                        string   `json:"engine"`
	ModelName                     string   `json:"model_name"`
	PolicyVersion                 string   `json:"policy_version"`
	Verdict                       string   `json:"verdict"`
	Confidence                    string   `json:"confidence"`
	ConfidenceScore               *float64 `json:"confidence_score"`
	SuggestedDefectType           string   `json:"suggested_defect_type"`
	SuggestedDefectTypeConfidence *float64 `json:"suggested_defect_type_confidence"`
	SuggestionSource              string   `json:"suggestion_source"`
	NarrativeStatus               string   `json:"narrative_status"`
	TokenUsagePrompt              int      `json:"token_usage_prompt"`
	TokenUsageCompletion          int      `json:"token_usage_completion"`
	TypeSafeInputTokens           int      `json:"typesafe_input_tokens"`
	DecisionStatus                string   `json:"decision_status"`
	ErrorCategory                 string   `json:"error_category"`
	JobID                         *string  `json:"job_id"`
	HistoryAvailable              bool     `json:"history_available"`
	SourceAnalysisID              *string  `json:"source_analysis_id"`
	Summary                       string   `json:"summary"`
	NextAction                    string   `json:"next_action"`
	Rationale                     string   `json:"rationale"`
}

// failedAttempt reports whether an analysis is a failed attempt rather than an answer.
// Servers from before decision_status stored a failed call as a generative row with no
// model name.
func failedAttempt(a Analysis) bool {
	if a.DecisionStatus != "" {
		return a.DecisionStatus == "failed"
	}
	return a.Engine == "generative" && a.ModelName == ""
}

// Job is one analysis job of the run (the wire shape of GET /api/runs/{id}/analysis-jobs).
type Job struct {
	ID              string       `json:"id"`
	Status          string       `json:"status"`
	PipelineLabel   string       `json:"pipeline_label"`
	RetryFailedOnly bool         `json:"retry_failed_only"`
	CreatedAt       string       `json:"created_at"`
	RateLimitHits   int          `json:"rate_limit_hits"`
	CallTimeouts    int          `json:"call_timeouts"`
	HedgesFired     int          `json:"hedges_fired"`
	HedgesWon       int          `json:"hedges_won"`
	Outcomes        *JobOutcomes `json:"outcomes"`
}

// JobOutcomes is the telemetry part of a job's outcomes: stage times in milliseconds over the
// job's representatives and the 429s, call timeouts and hedges seen. All zero on servers from
// before job telemetry.
type JobOutcomes struct {
	DecisionMsAvg int `json:"decision_ms_avg"`
	DecisionMsP50 int `json:"decision_ms_p50"`
	DecisionMsMax int `json:"decision_ms_max"`
	LLMMsAvg      int `json:"llm_ms_avg"`
	LLMMsP50      int `json:"llm_ms_p50"`
	LLMMsMax      int `json:"llm_ms_max"`
	RateLimitHits int `json:"rate_limit_hits"`
	CallTimeouts  int `json:"call_timeouts"`
	HedgesFired   int `json:"hedges_fired"`
	HedgesWon     int `json:"hedges_won"`
}

// jobTiming is the job's telemetry with each call count taken from whichever of the job row and
// its outcomes reports more; nil when the server sent none.
func jobTiming(j Job) *JobOutcomes {
	row := JobOutcomes{RateLimitHits: j.RateLimitHits, CallTimeouts: j.CallTimeouts,
		HedgesFired: j.HedgesFired, HedgesWon: j.HedgesWon}
	if j.Outcomes == nil {
		if row == (JobOutcomes{}) {
			return nil
		}
		return &row
	}
	t := *j.Outcomes
	t.RateLimitHits = max(t.RateLimitHits, row.RateLimitHits)
	t.CallTimeouts = max(t.CallTimeouts, row.CallTimeouts)
	t.HedgesFired = max(t.HedgesFired, row.HedgesFired)
	t.HedgesWon = max(t.HedgesWon, row.HedgesWon)
	return &t
}

func timingRecorded(t *JobOutcomes) bool {
	return t != nil && (t.DecisionMsMax > 0 || t.LLMMsMax > 0 || t.RateLimitHits > 0 ||
		t.CallTimeouts > 0 || t.HedgesFired > 0)
}

func durationText(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	return fmt.Sprintf("%.1f s", float64(ms)/1000)
}

// timingText renders one job's decision and LLM time, its rate-limit hits and, when any, its
// LLM call timeouts and hedges.
func timingText(t *JobOutcomes) string {
	if !timingRecorded(t) {
		return "timing not recorded"
	}
	stage := func(name string, avg, p50, hi int) string {
		if hi == 0 {
			return "no " + name + " time"
		}
		return fmt.Sprintf("%s avg %s · p50 %s · max %s", name, durationText(avg), durationText(p50), durationText(hi))
	}
	s := fmt.Sprintf("%s; %s; %d rate-limit hit(s)",
		stage("decision", t.DecisionMsAvg, t.DecisionMsP50, t.DecisionMsMax),
		stage("LLM", t.LLMMsAvg, t.LLMMsP50, t.LLMMsMax), t.RateLimitHits)
	if t.CallTimeouts > 0 {
		s += fmt.Sprintf("; %d call timeout(s)", t.CallTimeouts)
	}
	if t.HedgesFired > 0 {
		s += fmt.Sprintf("; %d hedge(s) fired, %d won", t.HedgesFired, t.HedgesWon)
	}
	return s
}

// ParseJobs reads GET /api/runs/{id}/analysis-jobs.
func ParseJobs(raw []byte) ([]Job, error) {
	var list []Job
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode analysis jobs: %w", err)
	}
	return list, nil
}

// ResolveJob finds a job by its id or a unique prefix of it (the by-job columns show
// the first eight characters).
func ResolveJob(jobs []Job, idOrPrefix string) (Job, error) {
	var hits []Job
	for _, j := range jobs {
		if j.ID == idOrPrefix {
			return j, nil
		}
		if strings.HasPrefix(j.ID, idOrPrefix) {
			hits = append(hits, j)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return Job{}, fmt.Errorf("no analysis job %q on this run", idOrPrefix)
	}
	return Job{}, fmt.Errorf("%q matches %d analysis jobs; give more of the id", idOrPrefix, len(hits))
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Options narrows or regroups the pivot. The zero value pivots every analysis by
// engine and model.
type Options struct {
	Job   string // keep only the analyses this job stored
	ByJob bool   // one column per job, labelled by its pipeline, instead of per engine/model
	Jobs  []Job  // the run's jobs: labels and order for by-job columns
	// DefectTypeMode is DefectTypeModeNative (or "") to grade the stored suggestions, or
	// DefectTypeModeMapping to re-grade every engine's defect type as its verdict's mapping.
	DefectTypeMode string
}

// Defect-type grading modes of `ttgo ai compare --defect-type-mode` (spec B5 #26).
const (
	DefectTypeModeNative  = "native"
	DefectTypeModeMapping = "mapping"
)

// ParseDefectTypeMode validates a --defect-type-mode value; "" means native.
func ParseDefectTypeMode(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", DefectTypeModeNative:
		return DefectTypeModeNative, nil
	case DefectTypeModeMapping:
		return DefectTypeModeMapping, nil
	}
	return "", fmt.Errorf("--defect-type-mode must be native or mapping, got %q", s)
}

// GroundTruth is one planted template of the AI demo dataset. An empty
// ExpectedVerdict means the template is graded on its defect type only.
type GroundTruth struct {
	TemplateKey        string `json:"template_key"`
	SampleMessage      string `json:"sample_message"`
	ExpectedVerdict    string `json:"expected_verdict"`
	ExpectedDefectType string `json:"expected_defect_type"`
}

// Column is one engine/model pair (and, for TypeSafe, one question-set policy
// version) that analyzed at least one row.
type Column struct {
	Key    string       `json:"key"`
	Engine string       `json:"engine"`
	Model  string       `json:"model"`
	Policy string       `json:"policy,omitempty"`
	Job    string       `json:"job,omitempty"`    // by-job columns: the job id
	Label  string       `json:"label,omitempty"`  // by-job columns: the job's pipeline
	Timing *JobOutcomes `json:"timing,omitempty"` // by-job columns: the job's telemetry
}

// Cell is the latest analysis of one row by one column.
type Cell struct {
	Version          int      `json:"version"`
	Verdict          string   `json:"verdict"`
	Confidence       string   `json:"confidence"`
	Score            *float64 `json:"confidence_score,omitempty"`
	DefectType       string   `json:"suggested_defect_type"`
	DefectScore      *float64 `json:"suggested_defect_type_confidence,omitempty"`
	SuggestionSource string   `json:"suggestion_source,omitempty"`
	NarrativeStatus  string   `json:"narrative_status,omitempty"`
	Tokens           int      `json:"tokens"`
	Failed           bool     `json:"failed,omitempty"` // a failed attempt: no decision
	ErrorCategory    string   `json:"error_category,omitempty"`
	History          bool     `json:"history_available"` // the analysis had this test's history
	Clone            bool     `json:"clone,omitempty"`   // a dedup clone: its representative's answer
	Summary          string   `json:"-"`
	NextAction       string   `json:"-"`
	Rationale        string   `json:"-"`
}

// Row is one failing result with its cell per column.
type Row struct {
	Result   Result           `json:"result"`
	Cells    map[string]*Cell `json:"cells"`
	Disagree bool             `json:"disagree"`
	Template string           `json:"template,omitempty"`
	Expected *GroundTruth     `json:"expected,omitempty"`
}

// Report is the pivoted run.
type Report struct {
	RunID   string   `json:"run_id"`
	RunName string   `json:"run_name"`
	Columns []Column `json:"columns"`
	Rows    []Row    `json:"rows"`
	// FailedCalls counts failed attempts that name no engine model (stored before
	// decision_status existed); they cannot be placed in a column. A failed attempt
	// that names its model is a failed cell of that column.
	FailedCalls int `json:"failed_calls"`
	// Job is the job the report is limited to (--job); JobLabel its pipeline.
	Job       string       `json:"job,omitempty"`
	JobLabel  string       `json:"job_label,omitempty"`
	JobTiming *JobOutcomes `json:"job_timing,omitempty"`
	// DefectTypeMode is "mapping" when defect types were re-graded from the verdicts.
	DefectTypeMode string `json:"defect_type_mode,omitempty"`
	// Excluded counts stored analyses left out because they belong to another job or,
	// grouped by job, to none (a single re-analyze).
	Excluded int `json:"excluded,omitempty"`
}

// Abstention is the suggested defect type TypeSafe returns when the evidence
// does not point at a source.
const Abstention = "insufficient_evidence"

// isAbstention reports whether a stored or expected defect type means "no
// suggestion": the analyzer stores a withheld or insufficient answer as "",
// the answer key writes an expected abstention as the human label
// to_investigate, and the question option itself is insufficient_evidence.
func isAbstention(defectType string) bool {
	switch defectType {
	case "", Abstention, "to_investigate":
		return true
	}
	return false
}

// defectMatch compares a suggestion with an expectation, treating every
// abstention spelling as the same answer.
func defectMatch(got, want string) bool {
	if isAbstention(want) {
		return isAbstention(got)
	}
	return got == want
}

var (
	uuidRe   = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	hexRe    = regexp.MustCompile(`\b[0-9a-f]{12,}\b`)
	digitsRe = regexp.MustCompile(`\d+`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

// Normalize reduces an error message to its template skeleton: ids, digit runs
// and timestamps become "#", whitespace collapses. Two messages from the same
// planted template normalize equal.
func Normalize(msg string) string {
	s := uuidRe.ReplaceAllString(msg, "#")
	s = hexRe.ReplaceAllString(s, "#")
	s = digitsRe.ReplaceAllString(s, "#")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func isFailing(status string) bool {
	switch strings.ToUpper(status) {
	case "FAIL", "ERROR":
		return true
	}
	return false
}

// ParseRun reads GET /api/runs/{id} and keeps the failing results in order.
func ParseRun(raw []byte) (name string, rows []Result, err error) {
	var run struct {
		Name    string `json:"name"`
		Results []struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			ErrorMessage string `json:"error_message"`
			TestCase     *struct {
				Name string `json:"name"`
			} `json:"test_case"`
		} `json:"run_results"`
	}
	if err := json.Unmarshal(raw, &run); err != nil {
		return "", nil, fmt.Errorf("decode run: %w", err)
	}
	for _, r := range run.Results {
		if !isFailing(r.Status) {
			continue
		}
		n := "(deleted test case)"
		if r.TestCase != nil && r.TestCase.Name != "" {
			n = r.TestCase.Name
		}
		rows = append(rows, Result{ID: r.ID, Name: n, Status: strings.ToUpper(r.Status), ErrorMessage: r.ErrorMessage})
	}
	return run.Name, rows, nil
}

// ParseAnalyses reads GET /api/run-results/{id}/analyses.
func ParseAnalyses(raw []byte) ([]Analysis, error) {
	var list []Analysis
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode analyses: %w", err)
	}
	return list, nil
}

// ParseGroundTruth accepts either a GET /api/seed/ai response (an object with
// "ground_truth") or a bare array, which is what a perfseed manifest's
// "ground_truth" field and a saved POST /api/seed/ai response both contain.
func ParseGroundTruth(raw []byte) ([]GroundTruth, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var list []GroundTruth
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode answer key: %w", err)
		}
		return list, nil
	}
	var obj struct {
		GroundTruth []GroundTruth `json:"ground_truth"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode answer key: %w", err)
	}
	return obj.GroundTruth, nil
}

// columnKey names the column an analysis belongs to. TypeSafe rows carry the
// question-set policy version, so a re-analysis under a new policy lands in
// its own column next to the old one instead of replacing it.
func columnKey(a Analysis) string {
	key := a.Engine + "/" + a.ModelName
	if a.PolicyVersion != "" {
		key += "@" + a.PolicyVersion
	}
	return key
}

// Pivot builds the report: one column per engine/model(/policy), one row per
// result, each cell the latest version that column produced for that row.
func Pivot(results []Result, analyses map[string][]Analysis) Report {
	return PivotWith(results, analyses, Options{})
}

// PivotWith is Pivot limited to one job (opts.Job) or with one column per job
// (opts.ByJob). A job's column holds everything that job stored, whichever engine
// decided each row (a takeover or a fallback puts an LLM answer in a TypeSafe job).
func PivotWith(results []Result, analyses map[string][]Analysis, opts Options) Report {
	rep := Report{Job: opts.Job}
	if opts.DefectTypeMode == DefectTypeModeMapping {
		rep.DefectTypeMode = DefectTypeModeMapping
	}
	labels := map[string]string{}
	order := map[string]int{}
	byID := map[string]Job{}
	for i, j := range opts.Jobs {
		labels[j.ID] = j.PipelineLabel
		order[j.ID] = i
		byID[j.ID] = j
	}
	rep.JobLabel = labels[opts.Job]
	if opts.Job != "" {
		rep.JobTiming = jobTiming(byID[opts.Job])
	}
	seen := map[string]Column{}
	for _, r := range results {
		row := Row{Result: r, Cells: map[string]*Cell{}}
		for _, a := range analyses[r.ID] {
			job := ""
			if a.JobID != nil {
				job = *a.JobID
			}
			if (opts.Job != "" && job != opts.Job) || (opts.ByJob && job == "") {
				rep.Excluded++
				continue
			}
			failed := failedAttempt(a)
			if failed && a.ModelName == "" && !opts.ByJob {
				rep.FailedCalls++
				continue
			}
			key := columnKey(a)
			col := Column{Key: key, Engine: a.Engine, Model: a.ModelName, Policy: a.PolicyVersion}
			if opts.ByJob {
				key = "job " + shortID(job)
				col = Column{Key: key, Job: job, Label: labels[job], Timing: jobTiming(byID[job])}
			}
			if cur := row.Cells[key]; cur != nil && cur.Version >= a.Version {
				continue
			}
			cell := &Cell{
				Version: a.Version, Verdict: a.Verdict, Confidence: a.Confidence, Score: a.ConfidenceScore,
				DefectType: a.SuggestedDefectType, DefectScore: a.SuggestedDefectTypeConfidence,
				SuggestionSource: a.SuggestionSource, NarrativeStatus: a.NarrativeStatus,
				Tokens: a.TypeSafeInputTokens + a.TokenUsagePrompt + a.TokenUsageCompletion,
				Failed: failed, ErrorCategory: a.ErrorCategory, History: a.HistoryAvailable,
				Clone: a.SourceAnalysisID != nil, Summary: a.Summary, NextAction: a.NextAction, Rationale: a.Rationale,
			}
			if rep.DefectTypeMode == DefectTypeModeMapping && !failed {
				// Like for like (spec B5 #26): every engine's suggestion becomes its verdict's
				// mapping, the rule the generative path already follows, so TypeSafe's separate
				// defect-type question no longer decides the comparison. No new calls.
				cell.DefectType = models.SuggestedDefectType(a.Verdict)
				cell.DefectScore, cell.SuggestionSource = nil, ""
			}
			row.Cells[key] = cell
			if _, ok := seen[key]; !ok {
				seen[key] = col
			}
		}
		row.Disagree = disagree(row.Cells)
		rep.Rows = append(rep.Rows, row)
	}
	for _, c := range seen {
		rep.Columns = append(rep.Columns, c)
	}
	sort.Slice(rep.Columns, func(i, j int) bool {
		a, b := rep.Columns[i], rep.Columns[j]
		if opts.ByJob {
			// Oldest job first (the API lists newest first); unknown jobs last.
			oa, okA := order[a.Job]
			ob, okB := order[b.Job]
			if okA != okB {
				return okA
			}
			if oa != ob {
				return oa > ob
			}
			return a.Key < b.Key
		}
		if (a.Engine == "typesafe") != (b.Engine == "typesafe") {
			return a.Engine == "typesafe"
		}
		if a.Engine != b.Engine {
			return a.Engine < b.Engine
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		return a.Policy < b.Policy
	})
	return rep
}

func disagree(cells map[string]*Cell) bool {
	first, n := "", 0
	for _, c := range cells {
		if c.Failed {
			continue // no answer to disagree with
		}
		if n == 0 {
			first = c.Verdict
		} else if c.Verdict != first {
			return true
		}
		n++
	}
	return false
}

// Grade attaches the matching template and expected labels to every row whose
// error message normalizes to a planted template's sample message.
func Grade(rep *Report, gt []GroundTruth) {
	byShape := map[string]GroundTruth{}
	for _, g := range gt {
		key := Normalize(g.SampleMessage)
		if _, dup := byShape[key]; !dup {
			byShape[key] = g
		}
	}
	for i := range rep.Rows {
		if g, ok := byShape[Normalize(rep.Rows[i].Result.ErrorMessage)]; ok {
			g := g
			rep.Rows[i].Template = g.TemplateKey
			rep.Rows[i].Expected = &g
		}
	}
}

// BucketStats is grading within one confidence label. Graded counts rows
// scored on defect type; VerdictGraded the subset whose key names a verdict.
type BucketStats struct {
	Graded         int `json:"graded"`
	VerdictGraded  int `json:"verdict_graded"`
	VerdictCorrect int `json:"verdict_correct"`
	DefectCorrect  int `json:"defect_correct"`
}

// HistoryStats is grading over the rows whose analysis had (or lacked) the test's history.
type HistoryStats struct {
	Graded         int `json:"graded"`
	VerdictGraded  int `json:"verdict_graded"`
	VerdictCorrect int `json:"verdict_correct"`
	DefectCorrect  int `json:"defect_correct"`
}

// ColumnStats summarizes one column.
type ColumnStats struct {
	Key            string                 `json:"key"`
	Analyzed       int                    `json:"analyzed"` // rows with a decision
	Failed         int                    `json:"failed"`   // rows whose latest attempt failed: not analyzed, not graded
	FailedGraded   int                    `json:"failed_graded"`
	Verdicts       map[string]int         `json:"verdicts"`
	Abstained      int                    `json:"abstained"`      // rows where the suggestion was withheld
	Issued         int                    `json:"issued"`         // rows with a suggestion
	Derived        int                    `json:"derived"`        // ...of which derived from the verdict (TypeSafe)
	IssuedGraded   int                    `json:"issued_graded"`  // graded rows with a suggestion
	IssuedCorrect  int                    `json:"issued_correct"` // ...of which correct (conditional accuracy)
	Scored         int                    `json:"scored"`
	MeanScore      float64                `json:"mean_confidence_score"`
	Tokens         int                    `json:"tokens"`
	Graded         int                    `json:"graded"`
	VerdictGraded  int                    `json:"verdict_graded"`
	VerdictCorrect int                    `json:"verdict_correct"`
	DefectCorrect  int                    `json:"defect_correct"`
	ByConfidence   map[string]BucketStats `json:"by_confidence"`
	WithHistory    HistoryStats           `json:"with_history"`    // graded rows analyzed with the test's history
	WithoutHistory HistoryStats           `json:"without_history"` // ...and without it (also every row analyzed before history_available existed)
}

// PairAgreement compares two columns on the rows both analyzed.
type PairAgreement struct {
	A            string `json:"a"`
	B            string `json:"b"`
	Compared     int    `json:"compared"`
	VerdictAgree int    `json:"verdict_agree"`
	DefectAgree  int    `json:"defect_agree"`
}

// Summary is the report's aggregate view.
type Summary struct {
	Rows     int             `json:"rows"`
	Ungraded int             `json:"ungraded"`
	Columns  []ColumnStats   `json:"columns"`
	Pairs    []PairAgreement `json:"pairs"`
}

// Summarize computes per-column statistics, grading and pairwise agreement.
func Summarize(rep Report) Summary {
	s := Summary{Rows: len(rep.Rows)}
	for _, col := range rep.Columns {
		cs := ColumnStats{Key: col.Key, Verdicts: map[string]int{}, ByConfidence: map[string]BucketStats{}}
		sum := 0.0
		for _, row := range rep.Rows {
			c := row.Cells[col.Key]
			if c == nil {
				continue
			}
			if c.Failed {
				cs.Failed++
				cs.Tokens += c.Tokens
				if row.Expected != nil {
					cs.FailedGraded++
				}
				continue
			}
			cs.Analyzed++
			cs.Verdicts[c.Verdict]++
			cs.Tokens += c.Tokens
			issued := !isAbstention(c.DefectType)
			if issued {
				cs.Issued++
				if c.SuggestionSource == "verdict" {
					cs.Derived++
				}
			} else {
				cs.Abstained++
			}
			if c.Score != nil {
				cs.Scored++
				sum += *c.Score
			}
			if row.Expected != nil {
				cs.Graded++
				h := &cs.WithoutHistory
				if c.History {
					h = &cs.WithHistory
				}
				h.Graded++
				b := cs.ByConfidence[c.Confidence]
				b.Graded++
				ok := defectMatch(c.DefectType, row.Expected.ExpectedDefectType)
				if ok {
					cs.DefectCorrect++
					b.DefectCorrect++
					h.DefectCorrect++
				}
				if issued {
					cs.IssuedGraded++
					if ok {
						cs.IssuedCorrect++
					}
				}
				if row.Expected.ExpectedVerdict != "" {
					cs.VerdictGraded++
					b.VerdictGraded++
					h.VerdictGraded++
					if c.Verdict == row.Expected.ExpectedVerdict {
						cs.VerdictCorrect++
						b.VerdictCorrect++
						h.VerdictCorrect++
					}
				}
				cs.ByConfidence[c.Confidence] = b
			}
		}
		if cs.Scored > 0 {
			cs.MeanScore = sum / float64(cs.Scored)
		}
		s.Columns = append(s.Columns, cs)
	}
	for _, row := range rep.Rows {
		if row.Expected == nil {
			s.Ungraded++
		}
	}
	for i := 0; i < len(rep.Columns); i++ {
		for j := i + 1; j < len(rep.Columns); j++ {
			p := PairAgreement{A: rep.Columns[i].Key, B: rep.Columns[j].Key}
			for _, row := range rep.Rows {
				a, b := row.Cells[p.A], row.Cells[p.B]
				if a == nil || b == nil || a.Failed || b.Failed {
					continue
				}
				p.Compared++
				if a.Verdict == b.Verdict {
					p.VerdictAgree++
				}
				if defectMatch(a.DefectType, b.DefectType) {
					p.DefectAgree++
				}
			}
			s.Pairs = append(s.Pairs, p)
		}
	}
	return s
}

func pct(n, d int) string {
	if d == 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", int(float64(n)*100/float64(d)+0.5))
}

func head(s string, n int) string {
	s = spaceRe.ReplaceAllString(s, " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func cellText(c *Cell) (verdict, defect string) {
	if c == nil {
		return "—", "—"
	}
	if c.Failed {
		cat := c.ErrorCategory
		if cat == "" {
			cat = "error"
		}
		return "FAILED (" + cat + ")", "—"
	}
	conf := c.Confidence
	if c.Score != nil {
		conf = fmt.Sprintf("%s %.0f%%", c.Confidence, *c.Score*100)
	}
	defect = c.DefectType
	if defect == "" {
		defect = "—"
	}
	if c.DefectScore != nil {
		defect = fmt.Sprintf("%s %.0f%%", defect, *c.DefectScore*100)
	}
	return fmt.Sprintf("%s (%s)", c.Verdict, conf), defect
}

// Render writes the human-readable table and summary.
func Render(w io.Writer, rep Report, s Summary) {
	graded := s.Ungraded < s.Rows
	fmt.Fprintf(w, "Run: %s (%s) — %d failing results, %d column(s)", rep.RunName, rep.RunID, s.Rows, len(rep.Columns))
	if rep.FailedCalls > 0 {
		fmt.Fprintf(w, "; %d stored analysis version(s) are failed calls that name no model and are not counted", rep.FailedCalls)
	}
	fmt.Fprintln(w)
	if rep.DefectTypeMode == DefectTypeModeMapping {
		fmt.Fprintln(w, "Defect types re-graded as each verdict's mapping (--defect-type-mode mapping); stored suggestions are ignored.")
	}
	if rep.Job != "" {
		label := rep.JobLabel
		if label == "" {
			label = "pipeline not recorded"
		}
		fmt.Fprintf(w, "Job: %s — %s\n", rep.Job, label)
		fmt.Fprintf(w, "  %s\n", timingText(rep.JobTiming))
	}
	for _, c := range rep.Columns {
		if c.Job != "" {
			label := c.Label
			if label == "" {
				label = "pipeline not recorded"
			}
			fmt.Fprintf(w, "%s = %s — %s\n", strings.ToUpper(c.Key), c.Job, label)
			fmt.Fprintf(w, "  %s\n", timingText(c.Timing))
		}
	}
	if rep.Excluded > 0 {
		fmt.Fprintf(w, "%d stored analysis version(s) belong to no selected job and are left out\n", rep.Excluded)
	}
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	hdr := []string{"", "TEST", "ERROR"}
	for _, c := range rep.Columns {
		hdr = append(hdr, strings.ToUpper(c.Key)+" VERDICT", "DEFECT")
	}
	if graded {
		hdr = append(hdr, "EXPECTED")
	}
	fmt.Fprintln(tw, strings.Join(hdr, "\t"))
	for _, row := range rep.Rows {
		mark := " "
		if row.Disagree {
			mark = "!"
		}
		line := []string{mark, head(row.Result.Name, 36), head(row.Result.ErrorMessage, 48)}
		for _, c := range rep.Columns {
			v, d := cellText(row.Cells[c.Key])
			line = append(line, v, d)
		}
		if graded {
			exp := "—"
			if row.Expected != nil {
				ev := row.Expected.ExpectedVerdict
				if ev == "" {
					ev = "any"
				}
				exp = ev + " / " + row.Expected.ExpectedDefectType
			}
			line = append(line, exp)
		}
		fmt.Fprintln(tw, strings.Join(line, "\t"))
	}
	tw.Flush()
	fmt.Fprintln(w)
	fmt.Fprintln(w, "! = the columns disagree on the verdict")
	fmt.Fprintln(w)
	for _, cs := range s.Columns {
		fmt.Fprintf(w, "%s: %d analyzed", cs.Key, cs.Analyzed)
		if cs.Failed > 0 {
			fmt.Fprintf(w, ", %d failed (no decision; not graded)", cs.Failed)
		}
		if cs.Scored > 0 {
			fmt.Fprintf(w, ", mean confidence %.0f%%", cs.MeanScore*100)
		}
		fmt.Fprintf(w, ", suggestions issued %d", cs.Issued)
		if cs.Derived > 0 {
			fmt.Fprintf(w, " (%d derived from the verdict)", cs.Derived)
		}
		fmt.Fprintf(w, " / withheld %d, %d tokens", cs.Abstained, cs.Tokens)
		if cs.Graded > 0 {
			fmt.Fprintf(w, "; verdict %s of %d, defect type %s of %d (issued: %s of %d)",
				pct(cs.VerdictCorrect, cs.VerdictGraded), cs.VerdictGraded, pct(cs.DefectCorrect, cs.Graded), cs.Graded,
				pct(cs.IssuedCorrect, cs.IssuedGraded), cs.IssuedGraded)
			for _, label := range []string{"high", "medium", "low"} {
				if b, ok := cs.ByConfidence[label]; ok {
					fmt.Fprintf(w, " · %s: verdict %s of %d, defect %s of %d",
						label, pct(b.VerdictCorrect, b.VerdictGraded), b.VerdictGraded, pct(b.DefectCorrect, b.Graded), b.Graded)
				}
			}
		}
		fmt.Fprintln(w)
		keys := make([]string, 0, len(cs.Verdicts))
		for k := range cs.Verdicts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %d", k, cs.Verdicts[k]))
		}
		fmt.Fprintf(w, "  verdicts: %s\n", strings.Join(parts, ", "))
		if cs.Graded > 0 {
			fmt.Fprintf(w, "  with history: verdict %s of %d, defect type %s of %d · without: verdict %s of %d, defect type %s of %d\n",
				pct(cs.WithHistory.VerdictCorrect, cs.WithHistory.VerdictGraded), cs.WithHistory.VerdictGraded,
				pct(cs.WithHistory.DefectCorrect, cs.WithHistory.Graded), cs.WithHistory.Graded,
				pct(cs.WithoutHistory.VerdictCorrect, cs.WithoutHistory.VerdictGraded), cs.WithoutHistory.VerdictGraded,
				pct(cs.WithoutHistory.DefectCorrect, cs.WithoutHistory.Graded), cs.WithoutHistory.Graded)
		}
	}
	for _, p := range s.Pairs {
		fmt.Fprintf(w, "\n%s vs %s: %d rows compared, verdict agreement %s, defect-type agreement %s\n",
			p.A, p.B, p.Compared, pct(p.VerdictAgree, p.Compared), pct(p.DefectAgree, p.Compared))
	}
	if graded && s.Ungraded > 0 {
		fmt.Fprintf(w, "\n%d row(s) did not match a planted template and were not graded.\n", s.Ungraded)
	}
}

// NoMatchNote is printed when an answer key was loaded but no row matched it.
const NoMatchNote = "Answer key loaded, but no failure matched a planted template, so nothing was graded. Is this run from the AI demo dataset?"

// GradingNote explains an answer key that graded nothing; empty otherwise.
func GradingNote(s Summary, attempted bool) string {
	if attempted && s.Rows > 0 && s.Ungraded == s.Rows {
		return NoMatchNote
	}
	return ""
}
