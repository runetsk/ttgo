package store

import (
	"strings"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateUser creates a new user with the given fields.
// Email is normalised to lowercase; a new UUID is assigned to ID.
func (s *Store) CreateUser(email, displayName, hashedPw, role string) (*models.User, error) {
	u := &models.User{
		ID:             uuid.New().String(),
		Email:          strings.ToLower(strings.TrimSpace(email)),
		DisplayName:    displayName,
		HashedPassword: hashedPw,
		Role:           role,
		Active:         true,
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, err
	}
	return u, nil
}

// FindUserByEmail performs a case-insensitive email lookup.
func (s *Store) FindUserByEmail(email string) (*models.User, error) {
	var u models.User
	err := s.db.Where("LOWER(email) = LOWER(?)", strings.TrimSpace(email)).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUser retrieves a user by primary key.
func (s *Store) GetUser(id string) (*models.User, error) {
	var u models.User
	err := s.db.First(&u, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers returns users ordered by created_at asc.
// If includeDeleted is false, users with Deleted=true are excluded.
func (s *Store) ListUsers(includeDeleted bool) ([]*models.User, error) {
	var users []*models.User
	q := s.db.Order("created_at asc")
	if !includeDeleted {
		q = q.Where("deleted = ?", false)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUser applies a partial update to a user and reloads the record.
func (s *Store) UpdateUser(id string, updates map[string]interface{}) (*models.User, error) {
	if err := s.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetUser(id)
}

// CountActiveAdmins returns the number of active users with role "admin".
func (s *Store) CountActiveAdmins() (int64, error) {
	var count int64
	err := s.db.Model(&models.User{}).Where("role = ? AND active = ?", "admin", true).Count(&count).Error
	return count, err
}

// CreateAuditLog persists an AuditLog entry.
// If entry.ID is empty, a new UUID is generated.
func (s *Store) CreateAuditLog(entry *models.AuditLog) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	return s.db.Create(entry).Error
}
