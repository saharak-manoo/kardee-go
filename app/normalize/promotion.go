package normalize

import (
	"strings"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

// Promotion runs every extractor on p: conditions and summaries, card eligibility and
// merchants. It returns the merchant rows to store for p.
func Promotion(cat *catalog.Catalog, p *models.Promotion) []models.PromotionMerchant {
	Apply(p, Extract(p.TitleTH+"\n"+p.DescriptionTH, p.DetailTH+"\n"+p.DetailEN))

	headline := strings.Join([]string{p.TitleTH, p.TitleEN, p.DescriptionTH, p.DescriptionEN}, "\n")
	detail := p.DetailTH + "\n" + p.DetailEN
	var cardTypes []string
	for _, c := range strings.Split(p.Cards, "|") {
		if c = strings.TrimSpace(c); c != "" {
			cardTypes = append(cardTypes, c)
		}
	}
	p.Eligibility = ExtractEligibility(cat, p.Bank, cardTypes, headline+"\n"+detail)

	// Conditions pages often list other shops in their footer; trust the headline first.
	ms := ExtractMerchants(cat, headline)
	if len(ms) == 0 {
		ms = ExtractMerchants(cat, detail)
	}
	rows := make([]models.PromotionMerchant, 0, len(ms))
	fuel := false
	for _, m := range ms {
		rows = append(rows, models.PromotionMerchant{PromotionID: p.ID, MerchantKey: m.Key, Match: m.Match})
		if mm, ok := cat.Merchant(m.Key); ok && mm.Category == "fuel" {
			fuel = true
		}
	}
	if fuel && (p.Category == "auto" || p.Category == "other" || p.Category == "") {
		p.Category = "fuel"
	}
	return rows
}
