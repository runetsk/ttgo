package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"ttgo/pkg/tracker/models"
)

// migrateDefects converts the legacy Jira-keyed defect_links table into the
// native defects + defect_links schema. Idempotent and transactional; MUST run
// before AutoMigrate touches defect_links.
func migrateDefects(db *gorm.DB) error {
	if db.Migrator().HasTable("defects") {
		return nil // already migrated
	}
	if !db.Migrator().HasTable("defect_links") || !hasColumn(db, "defect_links", "jira_issue_key") {
		return nil // fresh install (or already new schema) -> AutoMigrate creates tables
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`ALTER TABLE defect_links RENAME TO defect_links_legacy`).Error; err != nil {
			return fmt.Errorf("rename legacy defect_links: %w", err)
		}
		if err := tx.AutoMigrate(&models.Defect{}, &models.DefectLink{}); err != nil {
			return fmt.Errorf("create native defect tables: %w", err)
		}

		var rows []legacyDefectLink
		if err := tx.Table("defect_links_legacy").Scan(&rows).Error; err != nil {
			return fmt.Errorf("read legacy rows: %w", err)
		}
		tcSet := scanIDSet(tx, "test_cases")
		rrSet := scanIDSet(tx, "run_results")

		byKey := map[string]*models.Defect{}
		for _, r := range rows {
			if d, ok := byKey[r.JiraIssueKey]; ok {
				if r.CreatedAt.Before(d.CreatedAt) {
					d.CreatedAt = r.CreatedAt
				}
				if r.UpdatedAt.After(d.UpdatedAt) {
					d.UpdatedAt = r.UpdatedAt
				}
				continue
			}
			byKey[r.JiraIssueKey] = &models.Defect{
				ID:               uuid.New().String(),
				Title:            firstNonEmpty(r.LastKnownSummary, r.JiraIssueKey),
				Status:           legacyStatus(r.StatusCategory),
				Severity:         legacySeverity(r.LastKnownPriority),
				ExternalProvider: "Jira",
				ExternalKey:      r.JiraIssueKey,
				ExternalURL:      r.LastKnownURL,
				CreatedAt:        r.CreatedAt,
				UpdatedAt:        r.UpdatedAt,
			}
		}
		for _, d := range byKey {
			if err := tx.Create(d).Error; err != nil {
				return fmt.Errorf("insert migrated defect: %w", err)
			}
		}

		repaired := 0
		for _, r := range rows {
			link := models.DefectLink{ID: uuid.New().String(), DefectID: byKey[r.JiraIssueKey].ID, CreatedAt: r.CreatedAt}
			if r.TestCaseID != "" {
				if tcSet[r.TestCaseID] {
					tc := r.TestCaseID
					link.TestCaseID = &tc
				} else {
					repaired++
				}
			}
			if r.RunResultID != "" {
				if rrSet[r.RunResultID] {
					rr := r.RunResultID
					link.RunResultID = &rr
				} else {
					repaired++
				}
			}
			if link.TestCaseID == nil && link.RunResultID == nil {
				continue // drop fully-dangling orphan
			}
			if err := tx.Create(&link).Error; err != nil {
				return fmt.Errorf("insert migrated link: %w", err)
			}
		}
		if repaired > 0 {
			fmt.Printf("[migrate] repaired %d dangling defect-link FK(s)\n", repaired)
		}
		return tx.Exec(`DROP TABLE defect_links_legacy`).Error
	})
}

type legacyDefectLink struct {
	ID                string
	TestCaseID        string
	RunResultID       string
	JiraIssueKey      string
	LastKnownSummary  string
	LastKnownPriority string
	LastKnownURL      string
	StatusCategory    string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func hasColumn(db *gorm.DB, table, col string) bool {
	var n int64
	db.Raw(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(&n)
	return n > 0
}

func scanIDSet(tx *gorm.DB, table string) map[string]bool {
	var ids []string
	_ = tx.Table(table).Pluck("id", &ids).Error
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func legacyStatus(statusCategory string) string {
	if statusCategory == "done" {
		return "closed"
	}
	return "open"
}

func legacySeverity(priority string) string {
	switch priority {
	case "Highest":
		return "critical"
	case "High":
		return "major"
	case "Low", "Lowest":
		return "trivial"
	default:
		return "minor"
	}
}
