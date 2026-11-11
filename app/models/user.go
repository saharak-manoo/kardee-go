package models

import "time"

// User represents a Kardee user, identified by LINE User ID or internal ID.
type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	LineUserID string    `gorm:"size:64;uniqueIndex" json:"line_user_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserCard represents a credit card saved in the user's wallet.
type UserCard struct {
	UserID    uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	CardKey   string    `gorm:"primaryKey;size:100;index" json:"card_key"`
	CreatedAt time.Time `json:"created_at"`
}
