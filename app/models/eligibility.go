package models

// Eligibility scopes: which cards a rule covers.
const (
	ScopeBankAll = "bank_all" // every card of the promotion's bank
	ScopeCards   = "cards"    // the listed cards
	ScopeNetwork = "network"  // a network (and minimum tier), of one bank or any bank
)

// How sure we are about the eligibility we read.
const (
	ConfHigh   = "high"   // from structured bank data
	ConfMedium = "medium" // wording matched a known pattern
	ConfLow    = "low"    // nothing said about cards, or names we don't know
)

// EligibilityRule is one way to qualify. Rules of a promotion are OR'd.
type EligibilityRule struct {
	Scope    string   `json:"scope"`
	CardKeys []string `json:"card_keys,omitempty"`
	Network  string   `json:"network,omitempty"`
	MinTier  string   `json:"min_tier,omitempty"`
	Issuer   string   `json:"issuer,omitempty"` // "" = cards of any bank
}

// Eligibility says which cards can use a promotion, built by normalize.
type Eligibility struct {
	Rules      []EligibilityRule `json:"rules"`
	Excluded   []string          `json:"excluded,omitempty"` // card keys left out
	Confidence string            `json:"confidence"`
	Unmatched  []string          `json:"unmatched,omitempty"` // card names not in the catalog
}

// Merchant match kinds.
const (
	MatchMerchant = "merchant" // the promotion names the merchant
	MatchCategory = "category" // "participating stations": every merchant of the category
)

// PromotionMerchant ties a promotion to a catalog merchant.
type PromotionMerchant struct {
	PromotionID uint   `gorm:"primaryKey;autoIncrement:false"`
	MerchantKey string `gorm:"primaryKey;size:50;index"`
	Match       string `gorm:"size:10;not null"`
}
