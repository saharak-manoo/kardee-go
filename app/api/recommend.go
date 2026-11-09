package api

import (
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
	"github.com/saharak/kardee-go/app/recommend"
)

var errAmount = errors.New("amount must be a positive whole number of baht")

// parseCards splits "a,b" into normalized, distinct card keys in the user's order.
func parseCards(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		if k = catalog.NormKey(k); k != "" && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// parseAmount reads ?amount=; empty means not given.
func parseAmount(s string) (n int, given bool, err error) {
	if s = strings.TrimSpace(s); s == "" {
		return 0, false, nil
	}
	n, err = strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, true, errAmount
	}
	return n, true, nil
}

type cardView struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Bank     string `json:"bank"`
	BankName string `json:"bank_name"`
	Network  string `json:"network"`
	Tier     string `json:"tier,omitempty"`
	ApplyURL string `json:"apply_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Verified bool   `json:"verified"`
}

func viewCard(c catalog.Card, lang string) cardView {
	return cardView{
		Key: c.Key, Name: models.Text(lang, c.NameTH, c.NameEN), Bank: c.Bank,
		BankName: models.Banks[c.Bank].Get(lang), Network: c.Network, Tier: c.Tier,
		ApplyURL: c.ApplyURL, ImageURL: c.ImageURL, Verified: c.Verified,
	}
}

type merchantView struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

func viewMerchant(m catalog.Merchant, lang string) merchantView {
	return merchantView{Key: m.Key, Name: models.Text(lang, m.NameTH, m.NameEN), Category: m.Category}
}

func matches(q string, names ...string) bool {
	q = catalog.NormKey(q)
	return q == "" || slices.ContainsFunc(names, func(n string) bool { return strings.Contains(catalog.NormKey(n), q) })
}

func (s *Server) merchants(c fiber.Ctx) error {
	lang, q, cat := Lang(c), c.Query("q"), c.Query("category")
	out := []merchantView{}
	for _, m := range s.Catalog.Merchants {
		if (cat == "" || m.Category == cat) && matches(q, append([]string{m.Key, m.NameTH, m.NameEN}, m.Aliases...)...) {
			out = append(out, viewMerchant(m, lang))
		}
	}
	return c.JSON(fiber.Map{"lang": lang, "merchants": out})
}

func (s *Server) cards(c fiber.Ctx) error {
	lang, q, bank := Lang(c), c.Query("q"), c.Query("bank")
	out := []cardView{}
	for _, k := range s.Catalog.Cards {
		if (bank == "" || k.Bank == bank) && matches(q, append([]string{k.Key, k.NameTH, k.NameEN}, k.Aliases...)...) {
			out = append(out, viewCard(k, lang))
		}
	}
	return c.JSON(fiber.Map{"lang": lang, "cards": out})
}

func (s *Server) networks(c fiber.Ctx) error {
	lang := Lang(c)
	type networkView struct {
		Key   string   `json:"key"`
		Name  string   `json:"name"`
		Tiers []string `json:"tiers"`
	}
	out := make([]networkView, 0, len(s.Catalog.Networks))
	for _, n := range s.Catalog.Networks {
		out = append(out, networkView{n.Key, models.Text(lang, n.NameTH, n.NameEN), n.Tiers})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return c.JSON(fiber.Map{"lang": lang, "networks": out})
}

type itemView struct {
	ValueBaht     float64       `json:"value_baht"`
	UseCard       *cardView     `json:"use_card"`
	EligibleCards []string      `json:"eligible_cards"`
	Confidence    string        `json:"confidence"`
	MerchantMatch string        `json:"merchant_match"`
	NeedsMinSpend *int          `json:"needs_min_spend"`
	NeedsRegister bool          `json:"needs_register"`
	PromoCode     string        `json:"promo_code"`
	Promotion     PromotionView `json:"promotion"`
}

type suggestionView struct {
	Card            cardView      `json:"card"`
	ValueBaht       float64       `json:"value_baht"`
	ExtraVsBestHeld float64       `json:"extra_vs_best_held"`
	Promotion       PromotionView `json:"promotion"`
}

func viewItems(items []recommend.Item, lang string) []itemView {
	out := make([]itemView, len(items))
	for i, it := range items {
		v := itemView{
			ValueBaht: it.ValueBaht, EligibleCards: make([]string, len(it.EligibleCards)),
			Confidence: it.Confidence, MerchantMatch: it.MerchantMatch, NeedsMinSpend: it.NeedsMinSpend,
			NeedsRegister: it.Promotion.NeedsRegister, PromoCode: it.Promotion.PromoCode,
			Promotion: View(it.Promotion, lang, false),
		}
		for j, c := range it.EligibleCards {
			v.EligibleCards[j] = c.Key
		}
		if it.UseCard != nil {
			cv := viewCard(*it.UseCard, lang)
			v.UseCard = &cv
		}
		out[i] = v
	}
	return out
}

func (s *Server) recommend(c fiber.Ctx) error {
	lang := Lang(c)
	name := strings.TrimSpace(c.Query("merchant"))
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "merchant is required")
	}
	m, ok := s.Catalog.ResolveMerchant(name)
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "unknown merchant")
	}
	amount, given, err := parseAmount(c.Query("amount"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if !given {
		amount = recommend.DefaultAmount(m.Category)
	}

	var links []models.PromotionMerchant
	if err := s.DB.Where("merchant_key = ?", m.Key).Find(&links).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	match := map[uint]string{}
	idList := make([]uint, 0, len(links))
	for _, l := range links {
		match[l.PromotionID] = l.Match
		idList = append(idList, l.PromotionID)
	}
	var rows []models.Promotion
	if len(idList) > 0 {
		if err := s.DB.Where("id IN ? AND gone = ?", idList, false).Find(&rows).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	}
	cands := make([]recommend.Candidate, len(rows))
	for i := range rows {
		cands[i] = recommend.Candidate{Promotion: &rows[i], Match: match[rows[i].ID]}
	}

	res := recommend.Recommend(s.Catalog, recommend.Input{Cards: parseCards(c.Query("cards")), Amount: amount, Now: time.Now()}, cands)
	apply := make([]suggestionView, len(res.ConsiderApplying))
	for i, sg := range res.ConsiderApplying {
		apply[i] = suggestionView{viewCard(sg.Card, lang), sg.ValueBaht, sg.ExtraVsBestHeld, View(sg.Promotion, lang, false)}
	}
	unknown := res.UnknownCards
	if unknown == nil {
		unknown = []string{}
	}
	return c.JSON(fiber.Map{
		"lang":              lang,
		"merchant":          viewMerchant(m, lang),
		"amount":            amount,
		"amount_assumed":    !given,
		"best":              viewItems(res.Best, lang),
		"consider_applying": apply,
		"no_value":          viewItems(res.NoValue, lang),
		"unknown_cards":     unknown,
	})
}
