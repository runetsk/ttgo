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
)

// Result is one failing run result.
type Result struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}

// Analysis is one stored analysis version (the wire shape of
// GET /api/run-results/{id}/analyses, minus the narrative).
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
	NarrativeStatus               string   `json:"narrative_status"`
	TokenUsagePrompt              int      `json:"token_usage_prompt"`
	TokenUsageCompletion          int      `json:"token_usage_completion"`
	TypeSafeInputTokens           int      `json:"typesafe_input_tokens"`
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
	Key    string `json:"key"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Policy string `json:"policy,omitempty"`
}

// Cell is the latest analysis of one row by one column.
type Cell struct {
	Version         int      `json:"version"`
	Verdict         string   `json:"verdict"`
	Confidence      string   `json:"confidence"`
	Score           *float64 `json:"confidence_score,omitempty"`
	DefectType      string   `json:"suggested_defect_type"`
	DefectScore     *float64 `json:"suggested_defect_type_confidence,omitempty"`
	NarrativeStatus string   `json:"narrative_status,omitempty"`
	Tokens          int      `json:"tokens"`
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
	// FailedCalls counts stored analyses of failed LLM calls (no model name,
	// unknown verdict); they are nobody's answer and form no column.
	FailedCalls int `json:"failed_calls"`
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
	rep := Report{}
	seen := map[string]Column{}
	for _, r := range results {
		row := Row{Result: r, Cells: map[string]*Cell{}}
		for _, a := range analyses[r.ID] {
			if a.Engine == "generative" && a.ModelName == "" {
				rep.FailedCalls++
				continue
			}
			key := columnKey(a)
			if cur := row.Cells[key]; cur != nil && cur.Version >= a.Version {
				continue
			}
			row.Cells[key] = &Cell{
				Version: a.Version, Verdict: a.Verdict, Confidence: a.Confidence, Score: a.ConfidenceScore,
				DefectType: a.SuggestedDefectType, DefectScore: a.SuggestedDefectTypeConfidence,
				NarrativeStatus: a.NarrativeStatus,
				Tokens:          a.TypeSafeInputTokens + a.TokenUsagePrompt + a.TokenUsageCompletion,
			}
			if _, ok := seen[key]; !ok {
				seen[key] = Column{Key: key, Engine: a.Engine, Model: a.ModelName, Policy: a.PolicyVersion}
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

// ColumnStats summarizes one column.
type ColumnStats struct {
	Key            string                 `json:"key"`
	Analyzed       int                    `json:"analyzed"`
	Verdicts       map[string]int         `json:"verdicts"`
	Abstained      int                    `json:"abstained"`      // rows where the suggestion was withheld
	Issued         int                    `json:"issued"`         // rows with a suggestion
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
			cs.Analyzed++
			cs.Verdicts[c.Verdict]++
			cs.Tokens += c.Tokens
			issued := !isAbstention(c.DefectType)
			if issued {
				cs.Issued++
			} else {
				cs.Abstained++
			}
			if c.Score != nil {
				cs.Scored++
				sum += *c.Score
			}
			if row.Expected != nil {
				cs.Graded++
				b := cs.ByConfidence[c.Confidence]
				b.Graded++
				ok := defectMatch(c.DefectType, row.Expected.ExpectedDefectType)
				if ok {
					cs.DefectCorrect++
					b.DefectCorrect++
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
					if c.Verdict == row.Expected.ExpectedVerdict {
						cs.VerdictCorrect++
						b.VerdictCorrect++
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
				if a == nil || b == nil {
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
	fmt.Fprintf(w, "Run: %s (%s) — %d failing results, %d engine column(s)", rep.RunName, rep.RunID, s.Rows, len(rep.Columns))
	if rep.FailedCalls > 0 {
		fmt.Fprintf(w, "; %d stored analysis version(s) are failed LLM calls and are not counted", rep.FailedCalls)
	}
	fmt.Fprint(w, "\n\n")
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
	fmt.Fprintln(w, "! = the engines disagree on the verdict")
	fmt.Fprintln(w)
	for _, cs := range s.Columns {
		fmt.Fprintf(w, "%s: %d analyzed", cs.Key, cs.Analyzed)
		if cs.Scored > 0 {
			fmt.Fprintf(w, ", mean confidence %.0f%%", cs.MeanScore*100)
		}
		fmt.Fprintf(w, ", suggestions issued %d / withheld %d, %d tokens", cs.Issued, cs.Abstained, cs.Tokens)
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
