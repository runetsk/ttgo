package store

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"ttgo/pkg/tracker/models"
)

// ListComments returns all comments for a given target, ordered by created_at ASC.
func (s *Store) ListComments(targetType, targetID string) ([]models.Comment, error) {
	var comments []models.Comment
	err := s.db.Preload("User").
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

// CreateComment inserts a new comment.
func (s *Store) CreateComment(c *models.Comment) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.Content = strings.TrimSpace(c.Content)
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	if err := s.db.Create(c).Error; err != nil {
		return err
	}
	// Re-fetch with User preloaded so the response includes user_display_name
	return s.db.Preload("User").First(c, "id = ?", c.ID).Error
}

// GetComment fetches a single comment by ID.
func (s *Store) GetComment(id string) (*models.Comment, error) {
	var c models.Comment
	if err := s.db.First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateComment updates a comment's content.
func (s *Store) UpdateComment(id, content string) (*models.Comment, error) {
	content = strings.TrimSpace(content)
	now := time.Now()
	if err := s.db.Model(&models.Comment{}).Where("id = ?", id).
		Updates(map[string]interface{}{"content": content, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	var c models.Comment
	if err := s.db.Preload("User").First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteComment removes a comment by ID.
func (s *Store) DeleteComment(id string) error {
	return s.db.Delete(&models.Comment{}, "id = ?", id).Error
}

// CountCommentsByTargets returns a map of targetID -> comment count for the given target type and IDs.
func (s *Store) CountCommentsByTargets(targetType string, targetIDs []string) (map[string]int64, error) {
	type result struct {
		TargetID string
		Count    int64
	}
	var results []result
	err := s.db.Model(&models.Comment{}).
		Select("target_id, count(*) as count").
		Where("target_type = ? AND target_id IN ?", targetType, targetIDs).
		Group("target_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(results))
	for _, r := range results {
		m[r.TargetID] = r.Count
	}
	return m, nil
}

// CountDefectLinksByRuns returns open/closed defect counts per test-run ID.
func (s *Store) CountDefectLinksByRuns(runIDs []string) (open map[string]int, closed map[string]int, err error) {
	type row struct {
		TestRunID string
		Status    string
		N         int
	}
	var rows []row
	err = s.db.Raw(`
		SELECT rr.test_run_id, d.status, COUNT(DISTINCT d.id) as n
		FROM defect_links dl JOIN run_results rr ON rr.id = dl.run_result_id JOIN defects d ON d.id = dl.defect_id
		WHERE rr.test_run_id IN ? GROUP BY rr.test_run_id, d.status`, runIDs).Scan(&rows).Error
	open, closed = map[string]int{}, map[string]int{}
	for _, r := range rows {
		if r.Status == "closed" {
			closed[r.TestRunID] += r.N
		} else {
			open[r.TestRunID] += r.N
		}
	}
	return open, closed, err
}
