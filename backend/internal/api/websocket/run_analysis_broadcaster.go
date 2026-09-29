package websocket

import (
	"fmt"
	"ttgo/pkg/tracker/models"
)

// RunAnalysisBroadcaster adapts the Hub to the failureanalysis worker's Broadcaster interface.
type RunAnalysisBroadcaster struct {
	Hub *Hub
}

func (b *RunAnalysisBroadcaster) BroadcastRunAnalysisProgress(job *models.RunAnalysisJob, covered int) {
	b.Hub.Broadcast(NewEvent(EventRunAnalysisProgress, runTopic(job.TestRunID), map[string]interface{}{
		"job_id":           job.ID,
		"run_id":           job.TestRunID,
		"test_run_id":      job.TestRunID,
		"trigger":          job.Trigger,
		"status":           job.Status,
		"analyzed_groups":  job.AnalyzedCount,
		"unique_groups":    job.UniqueGroups,
		"capped_groups":    job.CappedAt,
		"covered_failures": covered,
		"total_failures":   job.TotalFailures,
	}))
}

func (b *RunAnalysisBroadcaster) BroadcastRunAnalysisCompleted(job *models.RunAnalysisJob, covered int) {
	b.Hub.Broadcast(NewEvent(EventRunAnalysisCompleted, runTopic(job.TestRunID), map[string]interface{}{
		"job_id":           job.ID,
		"run_id":           job.TestRunID,
		"test_run_id":      job.TestRunID,
		"status":           job.Status,
		"analyzed_groups":  job.AnalyzedCount,
		"unique_groups":    job.UniqueGroups,
		"capped_groups":    job.CappedAt,
		"covered_failures": covered,
		"total_failures":   job.TotalFailures,
	}))
}

// analysisPayload is the whole analysis the UI renders, shared by the .created and .updated
// events: a client merging a live event (analysisMeta.js mergeAnalysis) never needs a refetch
// and never replaces a fuller row with a partial one. analysis_id is kept for older clients.
func analysisPayload(a *models.RunResultAnalysis) map[string]interface{} {
	return map[string]interface{}{
		"id":                               a.ID,
		"analysis_id":                      a.ID,
		"run_result_id":                    a.RunResultID,
		"version":                          a.Version,
		"verdict":                          a.Verdict,
		"suggested_defect_type":            a.SuggestedDefectType,
		"suggested_defect_type_confidence": a.SuggestedDefectTypeConfidence,
		"suggestion_source":                a.SuggestionSource,
		"confidence":                       a.Confidence,
		"confidence_score":                 a.ConfidenceScore,
		"engine":                           a.Engine,
		"model_name":                       a.ModelName,
		"narrative_status":                 a.NarrativeStatus,
		"narrative_revision":               a.NarrativeRevision,
		"summary":                          a.Summary,
		"next_action":                      a.NextAction,
		"rationale":                        a.Rationale,
		"dedup_group_key":                  a.DedupGroupKey,
		"dedup_method":                     a.DedupMethod,
		"dedup_p_same":                     a.DedupPSame,
		"dedup_model":                      a.DedupModel,
		"dedup_policy_version":             a.DedupPolicyVersion,
		"source_analysis_id":               a.SourceAnalysisID,
		"decision_status":                  a.DecisionStatus,
		"error_category":                   a.ErrorCategory,
		"takeover_from_verdict":            a.TakeoverFromVerdict,
		"takeover_from_confidence":         a.TakeoverFromConfidence,
		"takeover_from_defect_type":        a.TakeoverFromDefectType,
		"policy_version":                   a.PolicyVersion,
		"history_available":                a.HistoryAvailable,
		"signals":                          a.Signals,
		"created_at":                       a.CreatedAt,
		"job_id":                           a.JobID,
	}
}

func (b *RunAnalysisBroadcaster) broadcastAnalysis(eventType string, a *models.RunResultAnalysis, testRunID string) {
	payload := analysisPayload(a)
	b.Hub.Broadcast(NewEvent(eventType, runResultTopic(a.RunResultID), payload))
	if testRunID != "" {
		b.Hub.Broadcast(NewEvent(eventType, runTopic(testRunID), payload))
	}
}

// BroadcastRunResultAnalysisCreated publishes a newly stored analysis version.
func (b *RunAnalysisBroadcaster) BroadcastRunResultAnalysisCreated(a *models.RunResultAnalysis, testRunID string) {
	b.broadcastAnalysis(EventRunResultAnalysisCreated, a, testRunID)
}

// BroadcastRunResultAnalysisUpdated publishes a change to a stored version's explanation (a
// narration landing, an Explain claim, a sweep). Same payload as .created.
func (b *RunAnalysisBroadcaster) BroadcastRunResultAnalysisUpdated(a *models.RunResultAnalysis, testRunID string) {
	b.broadcastAnalysis(EventRunResultAnalysisUpdated, a, testRunID)
}

func runTopic(runID string) string    { return fmt.Sprintf("run:%s", runID) }
func runResultTopic(id string) string { return fmt.Sprintf("run_result:%s", id) }
