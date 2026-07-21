package store

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// AISeedConfig controls the AI failure-analysis demo dataset: Days daily runs
// (newest = today) of ResultsPerRun results each, drawn from a TestCases
// catalog. Unlike the perf profile, failure CONTENT is the point here: results
// carry realistic error messages/stacks whose volatile parts (ids, timestamps,
// hex) normalize away under failureanalysis.Signature, so each template forms
// one dedup group per run instead of shattering or collapsing.
type AISeedConfig struct {
	Seed          uint64
	Days          int // one run per day, newest = today
	ResultsPerRun int
	TestCases     int
}

// DefaultAISeedConfig mirrors a mid-size project: 30 daily runs x 500 results.
func DefaultAISeedConfig() AISeedConfig {
	return AISeedConfig{Seed: 1, Days: 30, ResultsPerRun: 500, TestCases: 600}
}

// Scenario kinds for planted failure templates.
const (
	aiScenPersistent     = "persistent"      // open bug: same cases fail every run since start day
	aiScenFixed          = "fixed"           // historical bug: failed in a past window, then stopped
	aiScenIncident       = "incident"        // one historical day where ~35% of the run ERRORs
	aiScenLatestIncident = "latest-incident" // mass ERROR slice in the newest run only
	aiScenFlaky          = "flaky"           // dedicated cases fail intermittently (~30% of runs)
	aiScenBackground     = "background"      // rare failures sprinkled across normal cases
	aiScenSingleton      = "singleton"       // exactly one occurrence, newest run
)

// AISeedGroundTruth is the per-template answer key: what a correct AI analysis
// of a planted failure should conclude. Written into the seed manifest so
// verdicts can be graded against it after a real-LLM analysis pass.
type AISeedGroundTruth struct {
	TemplateKey     string `json:"template_key"`
	FailureType     string `json:"failure_type"`
	SampleMessage   string `json:"sample_message"`
	ExpectedVerdict string `json:"expected_verdict"`
	ExpectedDefect  string `json:"expected_defect_type"`
	Scenario        string `json:"scenario"`
	TotalRows       int    `json:"total_rows"`
	LatestRunRows   int    `json:"latest_run_rows"`
}

// AISeedResult reports what SeedAIFailureDataset created.
type AISeedResult struct {
	Folders     int
	Categories  int
	TestCases   int
	TestRuns    int
	RunResults  int
	FailingRows int
	LabeledRows int // historical failing rows carrying a conclusive human defect_type
	LatestRunID string
	RunIDs      []string // newest first
	GroundTruth []AISeedGroundTruth
}

// aiTemplate is one planted failure signature. message must keep its static
// text identical across instances and confine variability to parts the dedup
// normalizer strips: numbers of 4+ digits, 0x-hex, ISO timestamps, path dirs.
type aiTemplate struct {
	key         string
	failureType string
	verdict     string                 // ground-truth verdict (models.Verdict*)
	status      models.ExecutionStatus // FAIL or ERROR
	scenario    string
	cases       int // dedicated catalog cases (persistent/fixed/flaky scenarios)
	startAge    int // active when startAge >= run age >= endAge (days; persistent: endAge 0)
	endAge      int
	caseStem    string // test-case display name stem for dedicated cases
	message     func(rng *rand.Rand, ts string) string
	stack       func(rng *rand.Rand, msg string) string
}

// expectedDefect maps the ground-truth verdict to the human triage label used
// when planting historical labels. "unknown" plants to_investigate — humans
// could not classify it either.
func (t *aiTemplate) expectedDefect() string {
	if d := models.SuggestedDefectType(t.verdict); d != "" {
		return d
	}
	return "to_investigate"
}

func jsStack(msg, page, method, spec string, rng *rand.Rand) string {
	return fmt.Sprintf(
		"%s\n    at %s.%s (pages/%s.js:%d:%d)\n    at Context.<anonymous> (specs/%s:%d:%d)\n    at processTicksAndRejections (node:internal/process/task_queues:95:5)",
		msg, page, method, page, 40+rng.IntN(200), 3+rng.IntN(30), spec, 10+rng.IntN(120), 5+rng.IntN(12))
}

func netStack(msg string, rng *rand.Rand) string {
	return fmt.Sprintf(
		"Error: %s\n    at TCPConnectWrap.afterConnect [as oncomplete] (node:net:%d:%d)\n    at runNextTicks (node:internal/process/task_queues:60:5)",
		msg, 1200+rng.IntN(300), 10+rng.IntN(20))
}

// aiTemplates is the planted failure library. Message realism matters more
// than quantity: an LLM should be able to reach the ground-truth verdict from
// the message + stack + context alone.
var aiTemplates = []aiTemplate{
	{
		key: "checkout-total-mismatch", failureType: "assertion",
		verdict: models.VerdictProductBug, status: models.StatusFail,
		scenario: aiScenPersistent, cases: 6, startAge: 14, endAge: 0,
		caseStem: "Checkout — order total matches cart after discount",
		message: func(rng *rand.Rand, _ string) string {
			return fmt.Sprintf("AssertionError: expected order total 1499.00 to equal cart line total 1499.99 (order 5%04d)", rng.IntN(10000))
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "CheckoutPage", "assertOrderTotal", "checkout.spec.js", rng)
		},
	},
	{
		key: "profile-preferences-500", failureType: "http",
		verdict: models.VerdictProductBug, status: models.StatusFail,
		scenario: aiScenPersistent, cases: 4, startAge: 9, endAge: 0,
		caseStem: "Profile — save notification preferences",
		message: func(rng *rand.Rand, _ string) string {
			return fmt.Sprintf("POST /api/v1/profile/preferences returned 500 Internal Server Error (request req-8%07d)", rng.IntN(10000000))
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "ApiClient", "post", "profile.spec.js", rng)
		},
	},
	{
		key: "invoice-missing-currency", failureType: "assertion",
		verdict: models.VerdictProductBug, status: models.StatusFail,
		scenario: aiScenPersistent, cases: 3, startAge: 22, endAge: 0,
		caseStem: "Invoices — API response contract",
		message: func(_ *rand.Rand, _ string) string {
			return `Contract check failed: response missing required field "currency"; got fields [amount, subtotal, tax_total]`
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "InvoiceApi", "validateSchema", "invoices.spec.js", rng)
		},
	},
	{
		key: "export-csv-mojibake", failureType: "assertion",
		verdict: models.VerdictProductBug, status: models.StatusFail,
		scenario: aiScenFixed, cases: 6, startAge: 25, endAge: 10,
		caseStem: "Reports — CSV export round-trips UTF-8",
		message: func(_ *rand.Rand, _ string) string {
			return `Expected CSV header "Sépárátor test" to round-trip; got mojibake "SÃ©pÃ¡rÃ¡tor test"`
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "ReportsPage", "downloadCsv", "reports.spec.js", rng)
		},
	},
	{
		key: "stale-checkout-selector", failureType: "element",
		verdict: models.VerdictTestData, status: models.StatusFail,
		scenario: aiScenPersistent, cases: 5, startAge: 30, endAge: 0,
		caseStem: "Checkout — submit order via saved card",
		message: func(_ *rand.Rand, _ string) string {
			return `TimeoutError: locator('[data-test=checkout-submit]') not visible after 30000ms`
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "CheckoutPage", "submitOrder", "checkout.spec.js", rng)
		},
	},
	{
		key: "fixture-user-no-payment", failureType: "assertion",
		verdict: models.VerdictTestData, status: models.StatusFail,
		scenario: aiScenPersistent, cases: 3, startAge: 12, endAge: 0,
		caseStem: "Payments — pay with default saved method",
		message: func(_ *rand.Rand, _ string) string {
			return `Precondition failed: fixture user qa-checkout has no saved payment method; test requires at least one`
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "PaymentsPage", "requireSavedMethod", "payments.spec.js", rng)
		},
	},
	{
		key: "db-econnrefused", failureType: "network",
		verdict: models.VerdictInfrastructure, status: models.StatusError,
		scenario: aiScenIncident,
		message: func(_ *rand.Rand, _ string) string {
			return "connect ECONNREFUSED db-primary.staging.local:5432"
		},
		stack: func(rng *rand.Rand, msg string) string { return netStack(msg, rng) },
	},
	{
		key: "staging-nav-timeout", failureType: "timeout",
		verdict: models.VerdictEnvironment, status: models.StatusError,
		scenario: aiScenLatestIncident,
		message: func(_ *rand.Rand, ts string) string {
			return fmt.Sprintf("page.goto: Timeout 45000ms exceeded navigating to https://staging.ttgo.local/login (at %s)", ts)
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "LoginPage", "open", "smoke.spec.js", rng)
		},
	},
	{
		key: "toast-wait-timeout", failureType: "timeout",
		verdict: models.VerdictFlakyTest, status: models.StatusFail,
		scenario: aiScenFlaky, cases: 15,
		caseStem: "Notifications — toast confirms action",
		message: func(_ *rand.Rand, _ string) string {
			return "TimeoutError: waiting for selector .toast-success failed: timeout 10000ms exceeded"
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "ToastHelper", "waitForSuccess", "notifications.spec.js", rng)
		},
	},
	{
		key: "stale-element-detach", failureType: "element",
		verdict: models.VerdictFlakyTest, status: models.StatusFail,
		scenario: aiScenFlaky, cases: 15,
		caseStem: "Search — refine results by category",
		message: func(_ *rand.Rand, _ string) string {
			return "StaleElementReferenceError: element is not attached to the page document"
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "SearchPage", "applyFilter", "search.spec.js", rng)
		},
	},
	{
		key: "modal-opacity-race", failureType: "assertion",
		verdict: models.VerdictFlakyTest, status: models.StatusFail,
		scenario: aiScenFlaky, cases: 15,
		caseStem: "Auth — session expiry modal appears",
		message: func(rng *rand.Rand, _ string) string {
			return fmt.Sprintf("AssertionError: expected modal opacity 1 to be reached within animation budget; last observed 0.%04d", 1000+rng.IntN(8999))
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "SessionModal", "waitVisible", "auth.spec.js", rng)
		},
	},
	{
		key: "inventory-sku-missing", failureType: "assertion",
		verdict: models.VerdictTestData, status: models.StatusFail,
		scenario: aiScenBackground,
		message: func(rng *rand.Rand, _ string) string {
			return fmt.Sprintf("No rows matched filter sku=TTGO-8%04d in inventory export", rng.IntN(10000))
		},
		stack: func(rng *rand.Rand, msg string) string {
			return jsStack(msg, "InventoryPage", "findRow", "inventory.spec.js", rng)
		},
	},
	{
		key: "ws-close-1006", failureType: "network",
		verdict: models.VerdictUnknown, status: models.StatusFail,
		scenario: aiScenBackground,
		message: func(_ *rand.Rand, _ string) string {
			return "WebSocket closed unexpectedly during background sync (code 1006)"
		},
		stack: func(rng *rand.Rand, msg string) string { return netStack(msg, rng) },
	},
	{
		key: "runner-heap-oom", failureType: "exception",
		verdict: models.VerdictUnknown, status: models.StatusError,
		scenario: aiScenSingleton,
		message: func(rng *rand.Rand, _ string) string {
			return fmt.Sprintf("Runner process terminated: JavaScript heap out of memory (rss %d)", 2_000_000_000+rng.IntN(400_000_000))
		},
		stack: func(rng *rand.Rand, msg string) string { return netStack(msg, rng) },
	},
}

var aiAreas = [...]string{"Checkout", "Payments", "Profile", "Search", "Inventory", "Reports", "Auth", "Notifications"}

var aiCategories = [...]string{"smoke", "regression", "checkout", "api-contract", "ui-flows", "nightly"}

// aiSteps returns the per-area canned execution steps (stored shape: 0-based
// order_index, sanitized-HTML action/expected_result — see enrich.go stepDTO).
func aiSteps(area string) json.RawMessage {
	steps := []map[string]any{
		{"action": fmt.Sprintf("<p>Log in as the %s fixture user and open the %s section</p>", "qa-nightly", area), "expected_result": fmt.Sprintf("<p>%s landing page is visible</p>", area), "order_index": 0},
		{"action": "<p>Execute the scenario's primary flow with default test data</p>", "expected_result": "<p>Flow completes without validation errors</p>", "order_index": 1},
		{"action": "<p>Verify the resulting state via UI and API</p>", "expected_result": "<p>UI state and API payload agree with the expected snapshot</p>", "order_index": 2},
	}
	b, _ := json.Marshal(steps)
	return b
}

func aiLogText(msg, ts1, ts2 string) string {
	return fmt.Sprintf("[%s] [info] scenario started (worker 3, shard 2/4)\n[%s] [warn] retrying request once after transient 502 from cdn-edge\n[%s] [error] %s", ts1, ts1, ts2, msg)
}

// aiDataset holds the fully-generated AI demo entities before insertion, so
// the perfseed CLI path and the in-app demo path can share one generator but
// insert (and track) rows their own way.
type aiDataset struct {
	folders     []models.Folder
	categories  []models.Category
	cases       []models.TestCase
	assignments []models.CategoryTestCase
	runs        []models.TestRun
	results     []models.RunResult
	defects     []models.Defect
	links       []models.DefectLink
}

// buildAIFailureDataset generates the AI failure-analysis demo dataset. Row
// volume matches a real mid-size project while failure content is drawn from
// aiTemplates so the newest run contains a realistic spread of dedup groups,
// per-test-case history repeats across the trailing enrichment window, and
// older failing rows carry human triage labels for prompt grounding.
func buildAIFailureDataset(cfg AISeedConfig) (aiDataset, AISeedResult, error) {
	if cfg.Days < 7 || cfg.ResultsPerRun <= 0 || cfg.TestCases <= 0 {
		return aiDataset{}, AISeedResult{}, fmt.Errorf("days must be >= 7 and results-per-run/test-cases positive")
	}
	if cfg.ResultsPerRun > cfg.TestCases {
		return aiDataset{}, AISeedResult{}, fmt.Errorf(
			"results per run (%d) exceeds distinct test cases (%d): results must be unique per (run, case)",
			cfg.ResultsPerRun, cfg.TestCases)
	}

	// Dedicated-case role map: template index per case, -1 = normal.
	roles := make([]int, cfg.TestCases)
	for i := range roles {
		roles[i] = -1
	}
	next := 0
	for ti := range aiTemplates {
		for k := 0; k < aiTemplates[ti].cases; k++ {
			if next >= cfg.TestCases {
				return aiDataset{}, AISeedResult{}, fmt.Errorf("test cases (%d) too few for the template role map (%d needed)", cfg.TestCases, next+1)
			}
			roles[next] = ti
			next++
		}
	}
	// The newest run covers case indices [0, ResultsPerRun); the latest-run
	// specials must land inside it, past the dedicated-role block.
	latestIncLo, latestIncHi := cfg.ResultsPerRun-100, cfg.ResultsPerRun-75
	singletonCase := cfg.ResultsPerRun - 70
	if latestIncLo <= next {
		return aiDataset{}, AISeedResult{}, fmt.Errorf("results per run (%d) too small: latest-run incident slice would overlap the %d dedicated role cases", cfg.ResultsPerRun, next)
	}
	incidentIdx, latestIncidentIdx, singletonIdx := -1, -1, -1
	backgroundIdx := []int{}
	for ti := range aiTemplates {
		switch aiTemplates[ti].scenario {
		case aiScenIncident:
			incidentIdx = ti
		case aiScenLatestIncident:
			latestIncidentIdx = ti
		case aiScenSingleton:
			singletonIdx = ti
		case aiScenBackground:
			backgroundIdx = append(backgroundIdx, ti)
		}
	}
	const incidentAge = 6

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15))
	now := time.Now()

	// --- Folders / categories / catalog ------------------------------------
	rootID := perfID(cfg.Seed, "ai-folder-root", 0)
	folders := []models.Folder{{ID: rootID, Name: "AI Demo", CreatedAt: now, UpdatedAt: now}}
	areaIDs := make([]string, len(aiAreas))
	for i, area := range aiAreas {
		id := perfID(cfg.Seed, "ai-folder", i)
		areaIDs[i] = id
		parent := rootID
		folders = append(folders, models.Folder{ID: id, Name: area, ParentID: &parent, CreatedAt: now, UpdatedAt: now})
	}
	catIDs := make([]string, len(aiCategories))
	categories := make([]models.Category, 0, len(aiCategories))
	for i, c := range aiCategories {
		id := perfID(cfg.Seed, "ai-category", i)
		catIDs[i] = id
		categories = append(categories, models.Category{
			ID: id, Name: c, Description: "AI demo category", CreatedAt: now, UpdatedAt: now,
		})
	}

	roleSeq := make(map[int]int, len(aiTemplates)) // template -> running case counter for name suffixes
	cases := make([]models.TestCase, 0, cfg.TestCases)
	assignments := make([]models.CategoryTestCase, 0, cfg.TestCases)
	stepsByArea := make(map[int]json.RawMessage, len(aiAreas))
	for i := range aiAreas {
		stepsByArea[i] = aiSteps(aiAreas[i])
	}
	for i := 0; i < cfg.TestCases; i++ {
		id := perfID(cfg.Seed, "ai-case", i)
		area := i % len(aiAreas)
		name := fmt.Sprintf("%s — scenario %03d regression", aiAreas[area], i+1)
		if ti := roles[i]; ti >= 0 {
			roleSeq[ti]++
			name = fmt.Sprintf("%s #%d", aiTemplates[ti].caseStem, roleSeq[ti])
		}
		cases = append(cases, models.TestCase{
			ID: id, FolderID: areaIDs[area], Name: name,
			Description: fmt.Sprintf("Nightly regression coverage for the %s area. Exercises the primary user flow end-to-end including API contract checks.", aiAreas[area]),
			CreatedAt:   now.Add(-time.Duration(cfg.Days+30) * 24 * time.Hour), UpdatedAt: now,
		})
		assignments = append(assignments, models.CategoryTestCase{CategoryID: catIDs[i%len(catIDs)], TestCaseID: id})
	}

	// --- Runs + results -----------------------------------------------------
	gt := make([]AISeedGroundTruth, len(aiTemplates))
	for ti := range aiTemplates {
		t := &aiTemplates[ti]
		gt[ti] = AISeedGroundTruth{
			TemplateKey: t.key, FailureType: t.failureType,
			SampleMessage:   t.message(rand.New(rand.NewPCG(cfg.Seed, 7)), now.UTC().Format("2006-01-02T15:04:05Z")),
			ExpectedVerdict: t.verdict, ExpectedDefect: t.expectedDefect(), Scenario: t.scenario,
		}
	}

	runs := make([]models.TestRun, 0, cfg.Days)
	results := make([]models.RunResult, 0, cfg.Days*cfg.ResultsPerRun)
	runIDs := make([]string, 0, cfg.Days)
	failingRows, labeledRows := 0, 0

	for r := 0; r < cfg.Days; r++ {
		age := r // r=0 is today
		runID := perfID(cfg.Seed, "ai-run", r)
		runIDs = append(runIDs, runID)
		runCreated := now.Add(-time.Duration(age) * 24 * time.Hour)
		catID := catIDs[len(aiCategories)-1] // "nightly"
		runFailed := false
		incidentRun := age == incidentAge && incidentIdx >= 0

		for j := 0; j < cfg.ResultsPerRun; j++ {
			caseIdx := (r*17 + j) % cfg.TestCases
			start := runCreated.Add(time.Duration(j) * 700 * time.Millisecond)
			ts := start.UTC().Format("2006-01-02T15:04:05Z")

			ti := -1
			status := models.StatusPass
			switch {
			case incidentRun && rng.Float64() < 0.35:
				ti = incidentIdx
			case age == 0 && caseIdx >= latestIncLo && caseIdx < latestIncHi:
				ti = latestIncidentIdx
			case age == 0 && caseIdx == singletonCase:
				ti = singletonIdx
			case roles[caseIdx] >= 0:
				t := &aiTemplates[roles[caseIdx]]
				switch t.scenario {
				case aiScenFlaky:
					if rng.Float64() < 0.30 {
						ti = roles[caseIdx]
					}
				case aiScenPersistent:
					if age <= t.startAge {
						ti = roles[caseIdx]
					}
				case aiScenFixed:
					if age <= t.startAge && age >= t.endAge {
						ti = roles[caseIdx]
					}
				}
			case rng.Float64() < 0.008 && len(backgroundIdx) > 0:
				ti = backgroundIdx[caseIdx%len(backgroundIdx)]
			case rng.Float64() < 0.02:
				status = models.StatusSkip
			}

			res := models.RunResult{
				ID:               perfID(cfg.Seed, "ai-result", r*cfg.ResultsPerRun+j),
				TestRunID:        runID,
				AttemptNumber:    1,
				TestNameSnapshot: cases[caseIdx].Name,
				Status:           status,
				StartTime:        start,
				Browser:          perfBrowsers[j%len(perfBrowsers)],
				OS:               "linux",
				Environment:      "staging",
				AppVersion:       fmt.Sprintf("2.14.%d", cfg.Days-age),
				Steps:            stepsByArea[caseIdx%len(aiAreas)],
			}
			id := cases[caseIdx].ID
			res.TestCaseID = &id

			if ti >= 0 {
				t := &aiTemplates[ti]
				msg := t.message(rng, ts)
				res.Status = t.status
				res.ErrorMessage = msg
				res.StackTrace = t.stack(rng, msg)
				res.FailureType = t.failureType
				res.LogText = aiLogText(msg, start.Add(-40*time.Second).UTC().Format("2006-01-02T15:04:05Z"), ts)
				res.DurationMs = 100 + rng.Int64N(4900)
				if t.failureType == "timeout" {
					res.DurationMs = 30000 + rng.Int64N(16000)
				}
				// Human triage: older rows are mostly labeled with the ground
				// truth; the newest 3 runs are untriaged (to_investigate), which
				// is what the human-label rollup and calibration flows expect.
				res.DefectType = "to_investigate"
				if age >= 3 && rng.Float64() < 0.7 {
					res.DefectType = t.expectedDefect()
					if res.DefectType != "to_investigate" {
						labeledRows++
					}
				}
				runFailed = true
				failingRows++
				gt[ti].TotalRows++
				if age == 0 {
					gt[ti].LatestRunRows++
				}
			} else {
				res.DurationMs = 40 + rng.Int64N(2400)
			}
			res.EndTime = start.Add(time.Duration(res.DurationMs) * time.Millisecond)
			results = append(results, res)
		}

		runStatus := models.StatusPass
		if runFailed {
			runStatus = models.StatusFail
		}
		runs = append(runs, models.TestRun{
			ID: runID, Name: fmt.Sprintf("Nightly Regression — %s", runCreated.Format("2006-01-02")),
			CategoryID: &catID, Status: runStatus,
			CreatedAt: runCreated, UpdatedAt: runCreated,
		})
	}

	// --- Defects linked to planted product/automation bugs ------------------
	// Test-case-scoped links to the first dedicated case of a template, so the
	// enrichment's linked-defects lookup has real rows behind some failures.
	type defectSpec struct {
		templateKey, title, status, severity, key string
	}
	defectSpecs := []defectSpec{
		{"checkout-total-mismatch", "Order total drifts from cart total by payment rounding", "open", "major", "SHOP-1423"},
		{"invoice-missing-currency", "Invoice API dropped the currency field after the billing refactor", "open", "critical", "SHOP-1387"},
		{"export-csv-mojibake", "CSV export double-encodes UTF-8 headers", "closed", "minor", "SHOP-1129"},
	}
	firstCaseOfTemplate := make(map[string]string, len(aiTemplates))
	for i := 0; i < cfg.TestCases; i++ {
		if ti := roles[i]; ti >= 0 {
			if _, ok := firstCaseOfTemplate[aiTemplates[ti].key]; !ok {
				firstCaseOfTemplate[aiTemplates[ti].key] = cases[i].ID
			}
		}
	}
	defects := make([]models.Defect, 0, len(defectSpecs))
	links := make([]models.DefectLink, 0, len(defectSpecs))
	for i, d := range defectSpecs {
		caseID, ok := firstCaseOfTemplate[d.templateKey]
		if !ok {
			continue
		}
		defID := perfID(cfg.Seed, "ai-defect", i)
		defects = append(defects, models.Defect{
			ID: defID, Title: d.title, Status: d.status, Severity: d.severity,
			Description:      fmt.Sprintf("Filed from nightly regression triage; tracked externally as %s.", d.key),
			ExternalProvider: "jira", ExternalKey: d.key,
			ExternalURL: "https://issues.example.com/browse/" + d.key,
			CreatedAt:   now.Add(-10 * 24 * time.Hour), UpdatedAt: now,
		})
		links = append(links, models.DefectLink{
			ID: perfID(cfg.Seed, "ai-defect-link", i), DefectID: defID,
			TestCaseID: &caseID, CreatedAt: now.Add(-10 * 24 * time.Hour),
		})
	}

	ds := aiDataset{
		folders: folders, categories: categories, cases: cases,
		assignments: assignments, runs: runs, results: results,
		defects: defects, links: links,
	}
	return ds, AISeedResult{
		Folders:     len(folders),
		Categories:  len(categories),
		TestCases:   cfg.TestCases,
		TestRuns:    len(runs),
		RunResults:  len(results),
		FailingRows: failingRows, LabeledRows: labeledRows,
		LatestRunID: runIDs[0], RunIDs: runIDs, GroundTruth: gt,
	}, nil
}

// insertAIDataset writes a built dataset inside tx, parents before children so
// FK enforcement can never trip.
func insertAIDataset(tx *gorm.DB, ds aiDataset) error {
	if err := tx.CreateInBatches(&ds.folders, 200).Error; err != nil {
		return fmt.Errorf("insert folders: %w", err)
	}
	if err := tx.CreateInBatches(&ds.categories, 200).Error; err != nil {
		return fmt.Errorf("insert categories: %w", err)
	}
	if err := tx.CreateInBatches(&ds.cases, 200).Error; err != nil {
		return fmt.Errorf("insert test cases: %w", err)
	}
	if err := tx.CreateInBatches(&ds.assignments, 500).Error; err != nil {
		return fmt.Errorf("insert category assignments: %w", err)
	}
	if err := tx.CreateInBatches(&ds.runs, 200).Error; err != nil {
		return fmt.Errorf("insert runs: %w", err)
	}
	if err := tx.CreateInBatches(&ds.results, 500).Error; err != nil {
		return fmt.Errorf("insert results: %w", err)
	}
	if len(ds.defects) > 0 {
		if err := tx.CreateInBatches(&ds.defects, 50).Error; err != nil {
			return fmt.Errorf("insert defects: %w", err)
		}
		if err := tx.CreateInBatches(&ds.links, 50).Error; err != nil {
			return fmt.Errorf("insert defect links: %w", err)
		}
	}
	return nil
}

// SeedAIFailureDataset builds and inserts the AI failure-analysis dataset in
// one transaction — the perfseed CLI path (scratch DBs, no demo-seed tracking).
func (s *Store) SeedAIFailureDataset(cfg AISeedConfig) (AISeedResult, error) {
	ds, res, err := buildAIFailureDataset(cfg)
	if err != nil {
		return AISeedResult{}, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return insertAIDataset(tx, ds)
	}); err != nil {
		return AISeedResult{}, err
	}
	return res, nil
}
