package recommend

import (
	"slices"
	"testing"
	"time"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

const testCards = `
- {key: ktc-visa, bank: ktc, name_th: KTC Visa, network: visa, tier: platinum, apply_url: "https://ktc/apply"}
- {key: ktc-jcb, bank: ktc, name_th: KTC JCB, network: jcb, tier: ultimate}
- {key: ks-jcb-u, bank: krungsri, name_th: KS JCB U, network: jcb, tier: ultimate}
- {key: ks-jcb-g, bank: krungsri, name_th: KS JCB G, network: jcb, tier: gold}
- {key: ttb-a, bank: ttb, name_th: ttb A, network: visa, tier: signature, apply_url: "https://ttb/a"}
- {key: ttb-b, bank: ttb, name_th: ttb B, network: visa, tier: platinum}
- {key: kb-a, bank: kbank, name_th: KB A, network: visa, tier: gold, apply_url: "https://kb/a"}
- {key: kb-b, bank: kbank, name_th: KB B, network: mastercard, tier: world, apply_url: "https://kb/b"}
`

var now = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Parse([]byte(testCards), []byte("[]"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func card(t *testing.T, c *catalog.Catalog, key string) catalog.Card {
	t.Helper()
	k, ok := c.Card(key)
	if !ok {
		t.Fatalf("no card %s", key)
	}
	return k
}

func f(v float64) *float64 { return &v }
func i(v int) *int         { return &v }

// promo is an active, detailed promotion of bank with the given rules.
func promo(id uint, bank string, conf string, rules ...models.EligibilityRule) *models.Promotion {
	end := now.Add(30 * 24 * time.Hour)
	if len(rules) == 0 {
		rules = []models.EligibilityRule{{Scope: models.ScopeBankAll}}
	}
	return &models.Promotion{
		ID: id, Bank: bank, Category: "fuel", BenefitType: models.BenefitCashback,
		EndAt: &end, Eligibility: models.Eligibility{Rules: rules, Confidence: conf},
	}
}

func cashback(p *models.Promotion, pct float64, maxBenefit int) *models.Promotion {
	p.Percent, p.MaxBenefit = f(pct), i(maxBenefit)
	return p
}

func ids(items []Item) []uint {
	out := make([]uint, len(items))
	for n, it := range items {
		out[n] = it.Promotion.ID
	}
	return out
}

func cands(ps ...*models.Promotion) []Candidate {
	out := make([]Candidate, len(ps))
	for n, p := range ps {
		out[n] = Candidate{Promotion: p, Match: models.MatchMerchant}
	}
	return out
}

func TestValue(t *testing.T) {
	p := cashback(promo(1, "ktc", models.ConfHigh), 5, 100)
	if v, ok, _ := Value(p, 1500); !ok || v != 75 {
		t.Errorf("1500 → %v %v", v, ok)
	}
	if v, _, _ := Value(p, 3000); v != 100 {
		t.Errorf("3000 → %v, want capped 100", v)
	}
	p.MinSpend = i(2000)
	if v, ok, need := Value(p, 1500); !ok || v != 0 || need == nil || *need != 2000 {
		t.Errorf("below min → %v %v %v", v, ok, need)
	}
	off := promo(2, "ktc", models.ConfHigh)
	off.BenefitType, off.AmountOff = models.BenefitDiscount, i(200)
	if v, ok, _ := Value(off, 1500); !ok || v != 200 {
		t.Errorf("amount off → %v %v", v, ok)
	}
	inst := promo(3, "ktc", models.ConfHigh)
	inst.BenefitType = models.BenefitInstallment
	if _, ok, _ := Value(inst, 1500); ok {
		t.Error("installment has no value")
	}
}

func TestEligible(t *testing.T) {
	c := testCatalog(t)
	yes := func(key string, p *models.Promotion) bool { return Eligible(c, card(t, c, key), p) }

	all := promo(1, "ktc", models.ConfMedium)
	if !yes("ktc-visa", all) || yes("ttb-a", all) {
		t.Error("bank_all")
	}
	only := promo(2, "ttb", models.ConfMedium, models.EligibilityRule{Scope: models.ScopeCards, CardKeys: []string{"ttb-a"}})
	if !yes("ttb-a", only) || yes("ttb-b", only) {
		t.Error("cards")
	}
	jcb := promo(3, "jcb", models.ConfMedium, models.EligibilityRule{Scope: models.ScopeNetwork, Network: "jcb", MinTier: "platinum"})
	if !yes("ks-jcb-u", jcb) || yes("ks-jcb-g", jcb) || yes("ktc-visa", jcb) {
		t.Error("network any bank")
	}
	ktcJCB := promo(4, "ktc", models.ConfMedium, models.EligibilityRule{Scope: models.ScopeNetwork, Network: "jcb", Issuer: "ktc"})
	if !yes("ktc-jcb", ktcJCB) || yes("ks-jcb-u", ktcJCB) {
		t.Error("network with issuer")
	}
	excl := promo(5, "ktc", models.ConfMedium)
	excl.Eligibility.Excluded = []string{"ktc-visa"}
	if yes("ktc-visa", excl) || !yes("ktc-jcb", excl) {
		t.Error("excluded")
	}
	empty := promo(6, "ktc", models.ConfLow)
	empty.Eligibility.Rules = nil
	if !yes("ktc-jcb", empty) {
		t.Error("no rules = bank_all")
	}
}

func TestRecommend_RanksByValueThenConfidenceThenEnd(t *testing.T) {
	c := testCatalog(t)
	a := cashback(promo(1, "ktc", models.ConfMedium), 10, 100) // 100, medium
	b := cashback(promo(2, "ktc", models.ConfHigh), 10, 100)   // 100, high
	d := cashback(promo(3, "ktc", models.ConfHigh), 10, 50)    // 50
	e := cashback(promo(4, "ktc", models.ConfHigh), 10, 100)   // 100, high, ends sooner
	soon := now.Add(24 * time.Hour)
	e.EndAt = &soon
	r := Recommend(c, Input{Amount: 1500, Now: now}, cands(a, b, d, e))
	if got := ids(r.Best); !slices.Equal(got, []uint{4, 2, 1, 3}) {
		t.Errorf("order = %v", got)
	}
}

func TestRecommend_UseFirstHeldEligibleCard(t *testing.T) {
	c := testCatalog(t)
	r := Recommend(c, Input{Cards: []string{"ktc-jcb", "ktc-visa"}, Amount: 1500, Now: now},
		cands(cashback(promo(1, "ktc", models.ConfHigh), 5, 100)))
	if len(r.Best) != 1 || r.Best[0].UseCard == nil || r.Best[0].UseCard.Key != "ktc-jcb" {
		t.Errorf("best = %+v", r.Best)
	}
}

func TestRecommend_ConsiderApplyingThreshold(t *testing.T) {
	c := testCatalog(t)
	held := cashback(promo(1, "ktc", models.ConfHigh), 10, 100) // best held 100
	in := Input{Cards: []string{"ktc-visa"}, Amount: 2000, Now: now}

	near := cashback(promo(2, "ttb", models.ConfHigh), 10, 140) // +40
	if r := Recommend(c, in, cands(held, near)); len(r.ConsiderApplying) != 0 {
		t.Errorf("+40 suggested: %+v", r.ConsiderApplying)
	}

	far := cashback(promo(3, "ttb", models.ConfHigh), 10, 160) // +60
	r := Recommend(c, in, cands(held, far))
	if len(r.ConsiderApplying) != 1 || r.ConsiderApplying[0].ExtraVsBestHeld != 60 || r.ConsiderApplying[0].Card.Key != "ttb-a" {
		t.Errorf("+60: %+v", r.ConsiderApplying)
	}

	low := cashback(promo(4, "ttb", models.ConfLow), 50, 500)
	newCard := cashback(promo(5, "ttb", models.ConfHigh), 50, 500)
	newCard.Category = "new-card"
	if r := Recommend(c, in, cands(held, low, newCard)); len(r.ConsiderApplying) != 0 {
		t.Errorf("low/new-card suggested: %+v", r.ConsiderApplying)
	}

	k1 := cashback(promo(6, "ttb", models.ConfHigh), 50, 500)
	k2 := cashback(promo(7, "kbank", models.ConfHigh, models.EligibilityRule{Scope: models.ScopeCards, CardKeys: []string{"kb-a"}}), 50, 400)
	k3 := cashback(promo(8, "kbank", models.ConfHigh, models.EligibilityRule{Scope: models.ScopeCards, CardKeys: []string{"kb-b"}}), 50, 300)
	k4 := cashback(promo(9, "krungsri", models.ConfHigh), 50, 200)
	r = Recommend(c, in, cands(held, k1, k2, k3, k4))
	if len(r.ConsiderApplying) != 3 || r.ConsiderApplying[0].ValueBaht != 500 {
		t.Errorf("top 3: %+v", r.ConsiderApplying)
	}
}

func TestRecommend_SkipsUnknownDates(t *testing.T) {
	c := testCatalog(t)
	unknown := cashback(promo(1, "ktc", models.ConfHigh), 5, 100)
	unknown.EndAt = nil
	expired := cashback(promo(2, "ktc", models.ConfHigh), 5, 100)
	past := now.Add(-time.Hour)
	expired.EndAt = &past
	r := Recommend(c, Input{Amount: 1500, Now: now}, cands(unknown, expired))
	if len(r.Best)+len(r.NoValue) != 0 {
		t.Errorf("got %v / %v", ids(r.Best), ids(r.NoValue))
	}
}

func TestRecommend_NoHeldCardEligible(t *testing.T) {
	c := testCatalog(t)
	p := cashback(promo(1, "ttb", models.ConfHigh), 10, 200)
	r := Recommend(c, Input{Cards: []string{"ktc-visa", "ktc-visa"}, Amount: 2000, Now: now}, cands(p))
	if len(r.Best) != 0 || len(r.ConsiderApplying) == 0 {
		t.Errorf("best %v, apply %+v", ids(r.Best), r.ConsiderApplying)
	}
}

func TestRecommend_NoValueSeparated(t *testing.T) {
	c := testCatalog(t)
	g := promo(1, "ktc", models.ConfHigh)
	g.BenefitType = models.BenefitGift
	r := Recommend(c, Input{Cards: []string{"ktc-visa"}, Amount: 1500, Now: now}, cands(g))
	if len(r.Best) != 0 || !slices.Equal(ids(r.NoValue), []uint{1}) {
		t.Errorf("best %v novalue %v", ids(r.Best), ids(r.NoValue))
	}
}

func TestRecommend_UnknownCards(t *testing.T) {
	r := Recommend(testCatalog(t), Input{Cards: []string{"nope"}, Amount: 1500, Now: now}, nil)
	if !slices.Equal(r.UnknownCards, []string{"nope"}) {
		t.Errorf("unknown = %v", r.UnknownCards)
	}
}

func TestDefaultAmount(t *testing.T) {
	for cat, want := range map[string]int{"fuel": 1500, "dining": 1000, "shopping": 2000, "travel": 1000} {
		if got := DefaultAmount(cat); got != want {
			t.Errorf("%s = %d, want %d", cat, got, want)
		}
	}
}
