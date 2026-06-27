package api

import "context"

func (s *Server) StartBackupScheduler(ctx context.Context) {
	if s.backups == nil {
		return
	}
	s.backups.StartScheduler(ctx)
}
