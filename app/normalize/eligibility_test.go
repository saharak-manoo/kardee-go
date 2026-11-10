package normalize

import (
	"os"
	"slices"
	"testing"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hasRule(e models.Eligibility, want models.EligibilityRule) bool {
	return slices.ContainsFunc(e.Rules, func(r models.EligibilityRule) bool {
		return r.Scope == want.Scope && r.Network == want.Network && r.MinTier == want.MinTier &&
			r.Issuer == want.Issuer && slices.Equal(r.CardKeys, want.CardKeys)
	})
}

func TestEligibility_CrownDishJCBAnyBank(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "ktc", nil, fixture(t, "ktc_crown_dish_jcb.txt"))
	if !hasRule(e, models.EligibilityRule{Scope: models.ScopeNetwork, Network: "jcb", MinTier: "platinum"}) {
		t.Errorf("no any-bank JCB platinum rule: %+v", e.Rules)
	}
	if !hasRule(e, models.EligibilityRule{Scope: models.ScopeNetwork, Network: "jcb", Issuer: "ktc"}) {
		t.Errorf("no KTC JCB rule: %+v", e.Rules)
	}
}

func TestEligibility_KTCExclusions(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "ktc", nil, fixture(t, "ktc_donation_exclusions.txt"))
	if len(e.Rules) != 1 || e.Rules[0].Scope != models.ScopeBankAll {
		t.Errorf("rules = %+v, want bank_all", e.Rules)
	}
	for _, k := range []string{
		"ktc-cash-back-visa-platinum", "ktc-cash-back-platinum-mastercard", // "KTC CASH BACK ทุกประเภท"
		"ktc-royal-orchid-plus-visa-infinite", "ktc-royal-orchid-plus-jcb-platinum", // "KTC - ROYAL ORCHID PLUS ทุกประเภท"
		"ktc-visa-corporate", "ktc-government-services-card", "ktc-thai-red-cross-national-blood-center-platinum-mastercard",
	} {
		if !slices.Contains(e.Excluded, k) {
			t.Errorf("%s not excluded: %v", k, e.Excluded)
		}
	}
	if e.Confidence != models.ConfMedium {
		t.Errorf("confidence = %s", e.Confidence)
	}
}

func TestEligibility_TTBDisneyOnly(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "ttb", nil, "ดีลเอ็กซ์คลูซีฟ เฉพาะบัตร ทีทีบี ดิสนีย์ เท่านั้น")
	if !hasRule(e, models.EligibilityRule{Scope: models.ScopeCards, CardKeys: []string{"ttb-disney"}}) || len(e.Rules) != 1 {
		t.Errorf("rules = %+v", e.Rules)
	}
}

func TestEligibility_KrungsriCardTypes(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "krungsri",
		[]string{"บัตรเครดิต กรุงศรี เจซีบี อัลติเมท", "บัตรเครดิต โฮมโปร วีซ่า แพลทินัม", "บัตรเครดิต ไม่มีจริง"}, "")
	if !hasRule(e, models.EligibilityRule{Scope: models.ScopeCards, CardKeys: []string{"krungsri-jcb-ultimate", "homepro-visa-platinum"}}) {
		t.Errorf("rules = %+v", e.Rules)
	}
	if e.Confidence != models.ConfHigh {
		t.Errorf("confidence = %s", e.Confidence)
	}
	if !slices.Equal(e.Unmatched, []string{"บัตรเครดิต ไม่มีจริง"}) {
		t.Errorf("unmatched = %v", e.Unmatched)
	}
}

func TestEligibility_GenericTTB(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "ttb", nil, "เป็นเจ้าของ iPhone ด้วยบัตรเครดิต ttb")
	if len(e.Rules) != 1 || e.Rules[0].Scope != models.ScopeBankAll || e.Confidence != models.ConfMedium {
		t.Errorf("got %+v", e)
	}
}

func TestEligibility_NothingSaid(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "ktc", nil, "")
	if len(e.Rules) != 1 || e.Rules[0].Scope != models.ScopeBankAll || e.Confidence != models.ConfLow {
		t.Errorf("got %+v", e)
	}
}

func TestEligibility_VisaSource(t *testing.T) {
	e := ExtractEligibility(testCatalog(t), "visa", nil, "สำหรับบัตร Visa Signature และ Visa Infinite")
	if !hasRule(e, models.EligibilityRule{Scope: models.ScopeNetwork, Network: "visa", MinTier: "signature"}) || len(e.Rules) != 1 {
		t.Errorf("rules = %+v", e.Rules)
	}
}
