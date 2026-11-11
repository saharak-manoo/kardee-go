package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User is a Kardee user. Who they are on LINE, Google or Apple lives in UserIdentity, so
// one user can sign in from several apps. Kardee never stores card numbers, only which
// card products a user holds.
type User struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	DisplayName string         `gorm:"size:200" json:"display_name"`
	Locale      string         `gorm:"size:5;not null;default:th" json:"locale"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"` // account deleted on request (PDPA)
}

// UserIdentity is one login provider's account of a user.
type UserIdentity struct {
	ID          uint      `gorm:"primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	User        *User     `gorm:"constraint:OnDelete:CASCADE"`
	Provider    string    `gorm:"size:20;not null;uniqueIndex:idx_identity"` // auth.ProviderLINE …
	Subject     string    `gorm:"size:255;not null;uniqueIndex:idx_identity"`
	LastLoginAt time.Time
	CreatedAt   time.Time
}

// UserCard is a card product in the user's wallet. Position is the user's own order:
// the first eligible card is the one recommend tells them to use.
type UserCard struct {
	UserID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	User     *User     `gorm:"constraint:OnDelete:CASCADE"`
	CardKey  string    `gorm:"size:100;primaryKey"` // catalog key; keys never change
	Position int       `gorm:"not null"`
	AddedAt  time.Time `gorm:"not null"`
}

// Session is a signed-in client. Only the token's hash is stored.
type Session struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	User       *User     `gorm:"constraint:OnDelete:CASCADE"`
	TokenHash  string    `gorm:"size:64;not null;uniqueIndex"`
	Channel    string    `gorm:"size:10;not null"` // line, web, app
	ExpiresAt  time.Time `gorm:"not null;index"`
	LastUsedAt time.Time
	CreatedAt  time.Time
}

// SearchLog records one recommend request, to learn what people look for.
type SearchLog struct {
	ID           uint       `gorm:"primaryKey"`
	UserID       *uuid.UUID `gorm:"type:uuid;index"` // nil when not signed in
	Channel      string     `gorm:"size:10;not null"`
	MerchantKey  string     `gorm:"size:50;not null;index"`
	Amount       int        `gorm:"not null"`
	CardsCount   int        `gorm:"not null"`
	ResultsCount int        `gorm:"not null"` // best + no_value; 0 = we had nothing
	CreatedAt    time.Time  `gorm:"index"`
}

// AffiliateClick records a tap on "apply for this card".
type AffiliateClick struct {
	ID          uint       `gorm:"primaryKey"`
	UserID      *uuid.UUID `gorm:"type:uuid;index"`
	CardKey     string     `gorm:"size:100;not null;index"`
	PromotionID *uint      `gorm:"index"`
	Channel     string     `gorm:"size:10;not null"`
	CreatedAt   time.Time  `gorm:"index"`
}
