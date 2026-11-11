package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/saharak/kardee-go/app/catalog"
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

func TestCards_IncludeBenefits(t *testing.T) {
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	(&Server{Catalog: cat}).Routes(app)
	resp, err := app.Test(httptest.NewRequest("GET", "/v1/cards?q=ktc-x-visa-signature", nil))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Cards []struct {
			Key        string `json:"key"`
			DetailsURL string `json:"details_url"`
			MinIncome  int    `json:"min_income"`
			Benefits   []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Provider string `json:"provider"`
				Count    int    `json:"count"`
			} `json:"benefits"`
		} `json:"cards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Cards) != 1 || out.Cards[0].DetailsURL == "" || out.Cards[0].MinIncome != 50000 {
		t.Fatalf("%+v", out.Cards)
	}
	var lounge bool
	for _, b := range out.Cards[0].Benefits {
		lounge = lounge || b.Type == "lounge" && b.Provider == "MIRACLE LOUNGE" && b.Count == 2
	}
	if !lounge {
		t.Errorf("no MIRACLE LOUNGE benefit: %+v", out.Cards[0].Benefits)
	}
}
