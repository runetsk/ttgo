package store

import (
	"time"
	"ttgo/pkg/tracker/models"
)

// demoAIData groups the AI-analysis seed rows so the dataset aggregator can
// hand them off to the transaction in one place.
type demoAIData struct {
	Providers []models.LLMProviderConfig
	Analyses  []models.RunResultAnalysis
	Jobs      []models.RunAnalysisJob
}

// buildDemoAIData returns a seeded LLM provider plus a handful of analyses on
// failing results so the demo exercises the AI failure-analysis surface
// (verdicts, confidence, rationale, provider attribution, retry history).
func buildDemoAIData(now time.Time) demoAIData {
	providerID := demoID("llm:provider:demo-anthropic")
	providers := []models.LLMProviderConfig{
		{
			ID:                       providerID,
			Label:                    "Demo – Anthropic Claude",
			ProviderType:             "anthropic",
			EndpointURL:              "https://api.anthropic.com",
			APIKey:                   "demo-key-not-real-0000",
			ModelName:                "claude-opus-4-7",
			TimeoutSeconds:           90,
			IsDefault:                true,
			Enabled:                  true,
			AllowAutoFailureAnalysis: true,
			CreatedAt:                now,
			UpdatedAt:                now,
		},
	}

	pid := providerID
	analyses := []models.RunResultAnalysis{
		{
			ID:                   demoID("analysis:run2:tc9:v1"),
			RunResultID:          demoID("rr:run2-" + demoID("tc:session-expires")),
			Version:              1,
			Verdict:              models.VerdictProductBug,
			Confidence:           models.ConfidenceHigh,
			Summary:              "Dashboard header is not rendered after successful login on staging.",
			NextAction:           "Open a ticket for the auth team — the session appears valid but the header component crashes on mount.",
			Rationale:            "The logs show the login request returning 200 and setting the session cookie. The assertion failure is on an element that should render unconditionally for authenticated users, and the screenshot shows an empty page shell. This is not a test issue.",
			ModelName:            "claude-opus-4-7",
			ProviderID:           &pid,
			TokenUsagePrompt:     1180,
			TokenUsageCompletion: 320,
			CreatedAt:            now.Add(-30 * time.Minute),
		},
		{
			ID:                   demoID("analysis:run2:tc15:v1"),
			RunResultID:          demoID("rr:run2-" + demoID("tc:checkout-valid-payment")),
			Version:              1,
			Verdict:              models.VerdictEnvironment,
			Confidence:           models.ConfidenceMedium,
			Summary:              "Payment iframe failed to attach within 5s – likely a provider-side slowdown.",
			NextAction:           "Re-run on staging during off-peak hours before investigating code. Ping the payments vendor if it repeats.",
			Rationale:            "The waitForSelector timed out on #payment-frame. The same test passed in the previous sprint and no checkout code changed. A 5-second default is borderline on Firefox+Ubuntu runners.",
			ModelName:            "claude-opus-4-7",
			ProviderID:           &pid,
			TokenUsagePrompt:     1090,
			TokenUsageCompletion: 260,
			CreatedAt:            now.Add(-28 * time.Minute),
		},
		{
			ID:                   demoID("analysis:run3:tc11:v1"),
			RunResultID:          demoID("rr:run3-" + demoID("tc:account-lockout")),
			Version:              1,
			Verdict:              models.VerdictFlakyTest,
			Confidence:           models.ConfidenceMedium,
			Summary:              "Lockout banner selector is timing-sensitive and flakes under load.",
			NextAction:           "Replace the waitForSelector with a network-idle wait on the /login response, or increase the timeout to 10s.",
			Rationale:            "The lockout flow completes in the logs, but the banner element is rendered after a 300ms animation. Under load the 5s polling window sometimes misses the first render.",
			ModelName:            "claude-opus-4-7",
			ProviderID:           &pid,
			TokenUsagePrompt:     980,
			TokenUsageCompletion: 240,
			CreatedAt:            now.Add(-25 * time.Minute),
		},
		{
			ID:                   demoID("analysis:run3:tc11:v2"),
			RunResultID:          demoID("rr:run3-" + demoID("tc:account-lockout")),
			Version:              2,
			Verdict:              models.VerdictFlakyTest,
			Confidence:           models.ConfidenceHigh,
			Summary:              "Confirmed flaky: banner render race condition under load.",
			NextAction:           "Implement the animation-idle wait suggested in v1, and track flake rate after deploy.",
			Rationale:            "Re-ran with debug logging enabled. Captured two successful and one failed run at identical load, differing only in banner-render timing.",
			ModelName:            "claude-opus-4-7",
			ProviderID:           &pid,
			TokenUsagePrompt:     1040,
			TokenUsageCompletion: 310,
			CreatedAt:            now.Add(-10 * time.Minute),
		},
		{
			ID:                   demoID("analysis:run6:tc21:v1"),
			RunResultID:          demoID("rr:run6-" + demoID("tc:category-filter")),
			Version:              1,
			Verdict:              models.VerdictInfrastructure,
			Confidence:           models.ConfidenceHigh,
			Summary:              "Search API timed out – upstream network failure, not a test or product issue.",
			NextAction:           "No product action needed. Verify infra alerts fired and re-run once the search cluster is healthy.",
			Rationale:            "ERR_CONNECTION_TIMED_OUT on /api/search with a 30s wall time, while unrelated tests on the same page passed. Classic infra/transient network failure.",
			ModelName:            "claude-opus-4-7",
			ProviderID:           &pid,
			TokenUsagePrompt:     920,
			TokenUsageCompletion: 210,
			CreatedAt:            now.Add(-20 * time.Minute),
		},
	}

	startedRun2 := now.Add(-32 * time.Minute)
	completedRun2 := now.Add(-27 * time.Minute)
	startedRun3 := now.Add(-27 * time.Minute)
	completedRun3 := now.Add(-24 * time.Minute)
	startedRun6 := now.Add(-22 * time.Minute)
	completedRun6 := now.Add(-19 * time.Minute)

	jobs := []models.RunAnalysisJob{
		{
			ID:            demoID("aijob:run2"),
			TestRunID:     demoID("run:sprint1-regression"),
			Trigger:       models.RunAnalysisJobTriggerAutoOnDone,
			Status:        models.RunAnalysisJobStatusCompleted,
			TotalFailures: 3,
			UniqueGroups:  2,
			AnalyzedCount: 2,
			ModelName:     "claude-opus-4-7",
			ProviderID:    &pid,
			CreatedAt:     startedRun2,
			StartedAt:     &startedRun2,
			CompletedAt:   &completedRun2,
		},
		{
			ID:            demoID("aijob:run3"),
			TestRunID:     demoID("run:sprint1-edge"),
			Trigger:       models.RunAnalysisJobTriggerManual,
			Status:        models.RunAnalysisJobStatusCompleted,
			TotalFailures: 2,
			UniqueGroups:  1,
			AnalyzedCount: 1,
			ModelName:     "claude-opus-4-7",
			ProviderID:    &pid,
			CreatedAt:     startedRun3,
			StartedAt:     &startedRun3,
			CompletedAt:   &completedRun3,
		},
		{
			ID:            demoID("aijob:run6"),
			TestRunID:     demoID("run:sprint2-full"),
			Trigger:       models.RunAnalysisJobTriggerManual,
			Status:        models.RunAnalysisJobStatusCompleted,
			TotalFailures: 3,
			UniqueGroups:  3,
			AnalyzedCount: 1,
			CappedAt:      1,
			ModelName:     "claude-opus-4-7",
			ProviderID:    &pid,
			CreatedAt:     startedRun6,
			StartedAt:     &startedRun6,
			CompletedAt:   &completedRun6,
		},
	}

	return demoAIData{
		Providers: providers,
		Analyses:  analyses,
		Jobs:      jobs,
	}
}
