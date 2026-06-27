package models

import "time"

// TestCaseVersion stores a complete snapshot of a test case at a specific point in time.
// Every create or save operation produces one full snapshot regardless of how many fields changed.
type TestCaseVersion struct {
	ID                    string    `json:"id"                                 gorm:"primaryKey"`
	TestCaseID            string    `json:"test_case_id"                       gorm:"index;index:idx_version_tc_created;not null"`
	EventType             string    `json:"event_type"                         gorm:"not null"` // "create", "edit", "restore"
	RestoredFromVersionID string    `json:"restored_from_version_id,omitempty" gorm:"default:null"`
	UserID                string    `json:"user_id,omitempty"`
	UserName              string    `json:"user_name,omitempty"`
	Snapshot              string    `json:"snapshot"                           gorm:"type:text;not null"`
	CreatedAt             time.Time `json:"created_at"                         gorm:"index:idx_version_tc_created"`
}

// VersionSnapshot is the Go representation of the JSON stored in TestCaseVersion.Snapshot.
// It is used for serialization/deserialization only — not persisted as its own table.
type VersionSnapshot struct {
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Steps        []VersionStep        `json:"steps"`
	Categories   []VersionCategory    `json:"categories"`
	CustomValues []VersionCustomValue `json:"custom_values"`
}

// VersionStep captures a single step's state within a VersionSnapshot.
type VersionStep struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
	OrderIndex     int    `json:"order_index"`
}

// VersionCategory captures a category association within a VersionSnapshot.
type VersionCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// VersionCustomValue captures a custom field value within a VersionSnapshot.
type VersionCustomValue struct {
	FieldID   string `json:"field_id"`
	FieldName string `json:"field_name"`
	FieldType string `json:"field_type"`
	Value     string `json:"value"` // raw JSON string (e.g. "true", "42", "\"foo\"")
}
