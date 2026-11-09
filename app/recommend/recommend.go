// Package recommend ranks a merchant's promotions by what they are worth in baht,
// picks which of the user's cards to use, and finds cards worth applying for.
// It is pure: callers load the candidates.
package recommend

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

// Candidate is a promotion at the merchant, and how it was tied to it.
type Candidate struct {
	Promotion *models.Promotion
	Match     string // models.MatchMerchant or models.MatchCategory
}

type Input struct {
	Cards  []string // card keys the user holds, in their order
	Amount int      // baht
	Now    time.Time
}

type Item struct {
	Promotion     *models.Promotion
	ValueBaht     float64
	UseCard       *catalog.Card  // nil when no cards were given
	EligibleCards []catalog.Card // catalog cards that can use it
	Confidence    string
	MerchantMatch string
	NeedsMinSpend *int // set when Amount is below the minimum spend
}

type Suggestion struct {
	Card            catalog.Card
	ValueBaht       float64
	ExtraVsBestHeld float64
	Promotion       *models.Promotion
}

type Result struct {
	Best             []Item
	ConsiderApplying []Suggestion
	NoValue          []Item
	UnknownCards     []string
}

const (
	maxSuggestions = 3
	minExtraBaht   = 50
	minExtraShare  = 0.2
)

var defaultAmounts = map[string]int{"fuel": 1500, "dining": 1000, "shopping": 2000}

// DefaultAmount is the spend assumed when the user gives none.
func DefaultAmount(category string) int {
	if a, ok := defaultAmounts[category]; ok {
		return a
	}
	return 1000
}

// Value is what p is worth at amount baht. ok is false when it can't be computed
// (installments, gifts, points, nothing read).
func Value(p *models.Promotion, amount int) (baht float64, ok bool, needsMin *int) {
	switch {
	case (p.BenefitType == models.BenefitCashback || p.BenefitType == models.BenefitDiscount) && p.Percent != nil:
		baht = float64(amount) * *p.Percent / 100
		if p.MaxBenefit != nil {
			baht = math.Min(baht, float64(*p.MaxBenefit))
		}
	case p.AmountOff != nil:
		baht = float64(*p.AmountOff)
	default:
		return 0, false, nil
	}
	if p.MinSpend != nil && amount < *p.MinSpend {
		return 0, true, p.MinSpend
	}
	return baht, true, nil
}

// Eligible reports whether card c can use p.
func Eligible(cat *catalog.Catalog, c catalog.Card, p *models.Promotion) bool {
	e := p.Eligibility
	if slices.Contains(e.Excluded, c.Key) {
		return false
	}
	rules := e.Rules
	if len(rules) == 0 {
		rules = []models.EligibilityRule{{Scope: models.ScopeBankAll}}
	}
	for _, r := range rules {
		switch r.Scope {
		case models.ScopeBankAll:
			if c.Bank == p.Bank {
				return true
			}
		case models.ScopeCards:
			if slices.Contains(r.CardKeys, c.Key) {
				return true
			}
		case models.ScopeNetwork:
			if c.Network == r.Network && (r.Issuer == "" || c.Bank == r.Issuer) &&
				(r.MinTier == "" || cat.TierIndex(c.Network, c.Tier) >= cat.TierIndex(r.Network, r.MinTier)) {
				return true
			}
		}
	}
	return false
}

// live: running now, with a known end or a read detail page (same rule as the list API).
func live(p *models.Promotion, now time.Time) bool {
	return p.Active(now) && (p.EndAt != nil || p.DetailFetchedAt != nil)
}

func Recommend(cat *catalog.Catalog, in Input, cands []Candidate) Result {
	var res Result
	var held []catalog.Card
	for _, k := range in.Cards {
		c, ok := cat.Card(k)
		switch {
		case !ok:
			res.UnknownCards = append(res.UnknownCards, k)
		case !slices.ContainsFunc(held, func(h catalog.Card) bool { return h.Key == c.Key }):
			held = append(held, c)
		}
	}
	isHeld := func(key string) bool {
		return slices.ContainsFunc(held, func(h catalog.Card) bool { return h.Key == key })
	}

	var others []Item // usable only with cards the user doesn't hold
	for _, cd := range cands {
		p := cd.Promotion
		if !live(p, in.Now) {
			continue
		}
		it := Item{Promotion: p, Confidence: p.Eligibility.Confidence, MerchantMatch: cd.Match}
		for _, c := range cat.Cards {
			if Eligible(cat, c, p) {
				it.EligibleCards = append(it.EligibleCards, c)
			}
		}
		for _, h := range held {
			if Eligible(cat, h, p) {
				h := h
				it.UseCard = &h
				break
			}
		}
		v, ok, need := Value(p, in.Amount)
		it.ValueBaht, it.NeedsMinSpend = v, need
		if len(held) > 0 && it.UseCard == nil {
			if ok {
				others = append(others, it)
			}
			continue
		}
		if ok {
			res.Best = append(res.Best, it)
		} else {
			res.NoValue = append(res.NoValue, it)
		}
	}
	sortItems(res.Best)
	sortItems(res.NoValue)
	if len(held) > 0 {
		res.ConsiderApplying = suggest(res.Best, others, isHeld)
	}
	return res
}

// suggest picks cards the user doesn't hold whose best promotion beats their best by
// enough to matter.
func suggest(best, others []Item, isHeld func(string) bool) []Suggestion {
	bestHeld := 0.0
	if len(best) > 0 {
		bestHeld = best[0].ValueBaht
	}
	threshold := math.Max(minExtraBaht, minExtraShare*bestHeld)
	byCard := map[string]Suggestion{}
	for _, it := range others {
		if it.Confidence == models.ConfLow || it.Promotion.Category == "new-card" {
			continue
		}
		extra := it.ValueBaht - bestHeld
		if extra < threshold {
			continue
		}
		c, ok := pickCard(it.EligibleCards, isHeld)
		if !ok {
			continue
		}
		if s, seen := byCard[c.Key]; !seen || it.ValueBaht > s.ValueBaht {
			byCard[c.Key] = Suggestion{Card: c, ValueBaht: it.ValueBaht, ExtraVsBestHeld: extra, Promotion: it.Promotion}
		}
	}
	out := make([]Suggestion, 0, len(byCard))
	for _, s := range byCard {
		out = append(out, s)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].ValueBaht != out[b].ValueBaht {
			return out[a].ValueBaht > out[b].ValueBaht
		}
		return out[a].Card.Key < out[b].Card.Key
	})
	if len(out) > maxSuggestions {
		out = out[:maxSuggestions]
	}
	return out
}

// pickCard prefers a card the user can apply for online.
func pickCard(cards []catalog.Card, isHeld func(string) bool) (catalog.Card, bool) {
	var first *catalog.Card
	for n := range cards {
		c := &cards[n]
		if isHeld(c.Key) {
			continue
		}
		if c.ApplyURL != "" {
			return *c, true
		}
		if first == nil {
			first = c
		}
	}
	if first == nil {
		return catalog.Card{}, false
	}
	return *first, true
}

var confRank = map[string]int{models.ConfHigh: 0, models.ConfMedium: 1, models.ConfLow: 2}

func sortItems(items []Item) {
	sort.SliceStable(items, func(a, b int) bool {
		x, y := items[a], items[b]
		if x.ValueBaht != y.ValueBaht {
			return x.ValueBaht > y.ValueBaht
		}
		if rx, ry := rank(x.Confidence), rank(y.Confidence); rx != ry {
			return rx < ry
		}
		switch {
		case x.Promotion.EndAt == nil || y.Promotion.EndAt == nil:
			if (x.Promotion.EndAt == nil) != (y.Promotion.EndAt == nil) {
				return y.Promotion.EndAt == nil
			}
		case !x.Promotion.EndAt.Equal(*y.Promotion.EndAt):
			return x.Promotion.EndAt.Before(*y.Promotion.EndAt)
		}
		return x.Promotion.ID < y.Promotion.ID
	})
}

func rank(conf string) int {
	if r, ok := confRank[conf]; ok {
		return r
	}
	return confRank[models.ConfLow]
}
