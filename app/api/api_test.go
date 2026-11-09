package api

import (
	"testing"

	"github.com/saharak/kardee-go/app/models"
)

func TestView_EnglishFallsBackToThai(t *testing.T) {
	p := &models.Promotion{
		Bank: "krungsri", TitleTH: "ไทย", SummaryTH: "สรุป", SummaryEN: "summary",
		URLTH: "https://th", Category: "dining", BenefitType: models.BenefitCashback, Cards: "A|B",
	}
	v := View(p, models.LangEN, false)
	if v.Title != "ไทย" || v.TextLang != models.LangTH {
		t.Errorf("no English title: falls back to Thai and says so, got %q/%q", v.Title, v.TextLang)
	}
	if v.Summary != "summary" || v.BankName != "Krungsri" || v.CategoryName != "Dining" || v.BenefitName != "Cashback" {
		t.Errorf("%+v", v)
	}
	if len(v.Cards) != 2 || v.URL != "https://th" {
		t.Errorf("%+v", v)
	}

	p.TitleEN = "English"
	if v := View(p, models.LangEN, false); v.Title != "English" || v.TextLang != models.LangEN {
		t.Errorf("%+v", v)
	}
	if v := View(p, models.LangTH, false); v.Title != "ไทย" || v.BankName != "กรุงศรี" {
		t.Errorf("%+v", v)
	}
}
