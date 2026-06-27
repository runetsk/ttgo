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

func (b *RunAnalysisBroadcaster) BroadcastRunResultAnalysisCreated(a *models.RunResultAnalysis, testRunID string) {
	payload := map[string]interface{}{
		"run_result_id":   a.RunResultID,
		"analysis_id":     a.ID,
		"version":         a.Version,
		"verdict":         a.Verdict,
		"confidence":      a.Confidence,
		"dedup_group_key": a.DedupGroupKey,
	}
	b.Hub.Broadcast(NewEvent(EventRunResultAnalysisCreated, runResultTopic(a.RunResultID), payload))
	if testRunID != "" {
		b.Hub.Broadcast(NewEvent(EventRunResultAnalysisCreated, runTopic(testRunID), payload))
	}
}

func runTopic(runID string) string    { return fmt.Sprintf("run:%s", runID) }
func runResultTopic(id string) string { return fmt.Sprintf("run_result:%s", id) }
