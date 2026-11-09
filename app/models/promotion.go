// Package models holds the database tables.
package models

import (
	"time"
)

// Languages the app serves. Every user-facing text has a Thai and an English column;
// Thai is always filled, English when the bank publishes it or we can generate it.
const (
	LangTH = "th"
	LangEN = "en"
)

// Benefit types, stored as keys; labels per language live in Labels.
const (
	BenefitCashback    = "cashback"
	BenefitDiscount    = "discount"
	BenefitPoints      = "points"
	BenefitInstallment = "installment"
	BenefitGift        = "gift"
	BenefitOther       = "other"
)

// Promotion is one bank's credit card promotion.
type Promotion struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Bank       string `gorm:"size:20;not null;uniqueIndex:idx_bank_external" json:"bank"`
	ExternalID string `gorm:"size:300;not null;uniqueIndex:idx_bank_external" json:"external_id"`

	URLTH string `gorm:"size:500" json:"url_th"`
	URLEN string `gorm:"size:500" json:"url_en"`

	TitleTH       string `gorm:"type:text" json:"title_th"`
	TitleEN       string `gorm:"type:text" json:"title_en"`
	DescriptionTH string `gorm:"type:text" json:"description_th"`
	DescriptionEN string `gorm:"type:text" json:"description_en"`
	DetailTH      string `gorm:"type:text" json:"-"` // full conditions text as published
	DetailEN      string `gorm:"type:text" json:"-"`
	SummaryTH     string `gorm:"type:text" json:"summary_th"` // short, readable, built by normalize
	SummaryEN     string `gorm:"type:text" json:"summary_en"`

	Category string `gorm:"size:50;index" json:"category"` // our key, see Labels
	Cards    string `gorm:"type:text" json:"cards"`        // card names joined by "|"
	ImageURL string `gorm:"size:500" json:"image_url"`

	StartAt    *time.Time `gorm:"index" json:"start_at"`
	EndAt      *time.Time `gorm:"index" json:"end_at"`
	PeriodText string     `gorm:"size:200" json:"period_text"` // as the bank wrote it

	// Extracted conditions. nil = not found in the text.
	BenefitType   string   `gorm:"size:20;index" json:"benefit_type"`
	Percent       *float64 `json:"percent"`
	MinSpend      *int     `json:"min_spend"`   // baht
	MaxBenefit    *int     `json:"max_benefit"` // baht
	AmountOff     *int     `json:"amount_off"`  // baht off, when the discount is a fixed amount
	InstallMonths *int     `json:"install_months"`
	NeedsRegister bool     `json:"needs_register"`
	PromoCode     string   `gorm:"size:50" json:"promo_code"`

	// Which cards qualify, built by normalize from the catalog.
	Eligibility Eligibility `gorm:"type:jsonb;serializer:json" json:"eligibility"`

	// Crawl bookkeeping.
	SourceVersion   string     `gorm:"size:100" json:"-"` // changes when the bank edits it
	DetailVersion   string     `gorm:"size:100" json:"-"` // SourceVersion the detail was read at
	DetailFetchedAt *time.Time `json:"-"`
	LastSeenAt      time.Time  `gorm:"index" json:"last_seen_at"`
	Gone            bool       `gorm:"index" json:"-"` // no longer listed by the bank

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Text returns th or en, falling back to Thai when English is empty.
func Text(lang, th, en string) string {
	if lang == LangEN && en != "" {
		return en
	}
	return th
}

// Active reports whether the promotion runs at t.
func (p *Promotion) Active(t time.Time) bool {
	if p.Gone {
		return false
	}
	if p.StartAt != nil && t.Before(*p.StartAt) {
		return false
	}
	if p.EndAt != nil && t.After(*p.EndAt) {
		return false
	}
	return true
}

// CrawlRun records one crawl of one bank.
type CrawlRun struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Bank      string    `gorm:"size:20;index" json:"bank"`
	StartedAt time.Time `json:"started_at"`
	Duration  float64   `json:"duration_seconds"`
	Listed    int       `json:"listed"`
	New       int       `json:"new"`
	Details   int       `json:"details"`
	Gone      int       `json:"gone"`
	Error     string    `gorm:"type:text" json:"error"`
}
