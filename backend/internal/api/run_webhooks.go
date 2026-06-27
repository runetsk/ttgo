package api

import (
	"context"
	"log/slog"
	"ttgo/pkg/tracker/models"
)

func (s *Server) notifyRunCompleted(ctx context.Context, run *models.TestRun) {
	if s.webhookQueue == nil {
		return
	}

	event := &WebhookEvent{
		Event:      "run.completed",
		RunID:      run.ID,
		RunName:    run.Name,
		CategoryID: run.CategoryID,
		Status:     string(run.Status),
		CreatedAt:  run.CreatedAt,
	}

	select {
	case s.webhookQueue <- event:
	default:
		slog.WarnContext(ctx, "webhook queue full, dropping event", "run_id", run.ID)
	}
}
