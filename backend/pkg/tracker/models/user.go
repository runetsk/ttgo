package models

import "time"

// User represents a human with write access to the system.
type User struct {
	ID             string `json:"id"           gorm:"primaryKey"`
	Email          string `json:"email"        gorm:"uniqueIndex;not null"`
	DisplayName    string `json:"display_name"`
	HashedPassword string `json:"-"            gorm:"not null"`
	Role           string `json:"role"         gorm:"not null;default:'member'"`
	Active         bool   `json:"active"       gorm:"not null;default:true"`
	Deleted        bool   `json:"deleted"      gorm:"not null;default:false"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserSession represents a server-side session linking an active browser session
// to a User. The ID field is the opaque session token (32 random bytes, hex-encoded).
type UserSession struct {
	ID         string    `json:"id"          gorm:"primaryKey"` // 64-char hex token
	UserID     string    `json:"user_id"     gorm:"index;not null"`
	User       User      `json:"-"           gorm:"foreignKey:UserID"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"  gorm:"index"`
}
