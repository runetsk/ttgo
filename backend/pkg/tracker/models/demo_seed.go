package models

import "time"

// DemoSeed records which entities were created by the demo seed operation.
// It enables targeted removal of demo data without touching user-created content.
type DemoSeed struct {
	ID         string    `json:"id"          gorm:"primaryKey"`
	EntityType string    `json:"entity_type" gorm:"index:idx_demo_seed_type_entity;not null"` // folder|category|run_folder|test_case|test_run
	EntityID   string    `json:"entity_id"   gorm:"index:idx_demo_seed_type_entity;not null"`
	SeededAt   time.Time `json:"seeded_at"`
}
