// Package api serves promotions as JSON. Every endpoint takes ?lang=th|en (or the
// Accept-Language header); English falls back to Thai where the bank has no English.
package api

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/models"
)

type Server struct {
	DB      *gorm.DB
	Catalog *catalog.Catalog
}

func (s *Server) Routes(app *fiber.App) {
	app.Get("/health", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	v1 := app.Group("/v1")
	v1.Get("/banks", s.banks)
	v1.Get("/categories", s.categories)
	v1.Get("/promotions", s.promotions)
	v1.Get("/promotions/:id", s.promotion)
	v1.Get("/crawl-runs", s.crawlRuns)
	v1.Get("/merchants", s.merchants)
	v1.Get("/cards", s.cards)
	v1.Get("/networks", s.networks)
	v1.Get("/recommend", s.recommend)
}

// Lang picks the response language: ?lang= first, then Accept-Language, else Thai.
func Lang(c fiber.Ctx) string {
	if l := strings.ToLower(c.Query("lang")); l == models.LangEN || l == models.LangTH {
		return l
	}
	if strings.HasPrefix(strings.ToLower(c.Get("Accept-Language")), "en") {
		return models.LangEN
	}
	return models.LangTH
}

type option struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func options(m map[string]models.Label, lang string) []option {
	out := make([]option, 0, len(m))
	for k, v := range m {
		out = append(out, option{Key: k, Name: v.Get(lang)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func (s *Server) banks(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"lang": Lang(c), "banks": options(models.Banks, Lang(c))})
}

func (s *Server) categories(c fiber.Ctx) error {
	lang := Lang(c)
	return c.JSON(fiber.Map{
		"lang":          lang,
		"categories":    options(models.Categories, lang),
		"benefit_types": options(models.BenefitTypes, lang),
	})
}

// PromotionView is one promotion in the requested language.
type PromotionView struct {
	ID            uint       `json:"id"`
	Bank          string     `json:"bank"`
	BankName      string     `json:"bank_name"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Summary       string     `json:"summary"`
	Detail        string     `json:"detail,omitempty"`
	Category      string     `json:"category"`
	CategoryName  string     `json:"category_name"`
	BenefitType   string     `json:"benefit_type"`
	BenefitName   string     `json:"benefit_name"`
	Percent       *float64   `json:"percent"`
	MinSpend      *int       `json:"min_spend"`
	MaxBenefit    *int       `json:"max_benefit"`
	InstallMonths *int       `json:"install_months"`
	NeedsRegister bool       `json:"needs_register"`
	PromoCode     string     `json:"promo_code,omitempty"`
	Cards         []string   `json:"cards"`
	ImageURL      string     `json:"image_url,omitempty"`
	URL           string     `json:"url"`
	StartAt       *time.Time `json:"start_at"`
	EndAt         *time.Time `json:"end_at"`
	PeriodText    string     `json:"period_text,omitempty"`
	Active        bool       `json:"active"`
	// TextLang says which language the texts are really in ("th" when English was asked
	// for but the bank only publishes Thai).
	TextLang string `json:"text_lang"`
}

// View localizes p.
func View(p *models.Promotion, lang string, withDetail bool) PromotionView {
	v := PromotionView{
		ID: p.ID, Bank: p.Bank, BankName: models.Banks[p.Bank].Get(lang),
		Title:       models.Text(lang, p.TitleTH, p.TitleEN),
		Description: models.Text(lang, p.DescriptionTH, p.DescriptionEN),
		Summary:     models.Text(lang, p.SummaryTH, p.SummaryEN),
		Category:    p.Category, CategoryName: models.Categories[p.Category].Get(lang),
		BenefitType: p.BenefitType, BenefitName: models.BenefitTypes[p.BenefitType].Get(lang),
		Percent: p.Percent, MinSpend: p.MinSpend, MaxBenefit: p.MaxBenefit, InstallMonths: p.InstallMonths,
		NeedsRegister: p.NeedsRegister, PromoCode: p.PromoCode,
		Cards:    []string{},
		ImageURL: p.ImageURL, URL: models.Text(lang, p.URLTH, p.URLEN),
		StartAt: p.StartAt, EndAt: p.EndAt, PeriodText: p.PeriodText,
		Active:   p.Active(time.Now()),
		TextLang: models.LangTH,
	}
	if lang == models.LangEN && p.TitleEN != "" {
		v.TextLang = models.LangEN
	}
	if p.Cards != "" {
		v.Cards = strings.Split(p.Cards, "|")
	}
	if withDetail {
		v.Detail = models.Text(lang, p.DetailTH, p.DetailEN)
	}
	return v
}

func (s *Server) promotions(c fiber.Ctx) error {
	lang := Lang(c)
	q := s.DB.Model(&models.Promotion{}).Where("gone = ?", false)
	if b := c.Query("bank"); b != "" {
		q = q.Where("bank IN ?", strings.Split(b, ","))
	}
	if cat := c.Query("category"); cat != "" {
		q = q.Where("category IN ?", strings.Split(cat, ","))
	}
	if bt := c.Query("benefit"); bt != "" {
		q = q.Where("benefit_type IN ?", strings.Split(bt, ","))
	}
	if c.Query("active", "true") != "false" {
		now := time.Now()
		// Unknown dates (KTC lists a page before we read it) are not shown as active.
		q = q.Where("(start_at IS NULL OR start_at <= ?) AND (end_at IS NULL OR end_at >= ?)", now, now).
			Where("end_at IS NOT NULL OR detail_fetched_at IS NOT NULL")
	}
	if text := strings.TrimSpace(c.Query("q")); text != "" {
		like := "%" + strings.NewReplacer("%", `\%`, "_", `\_`).Replace(text) + "%"
		q = q.Where("title_th ILIKE ? OR title_en ILIKE ? OR description_th ILIKE ? OR description_en ILIKE ? OR detail_th ILIKE ?",
			like, like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	switch c.Query("sort", "ending") {
	case "newest":
		q = q.Order("created_at DESC")
	case "percent":
		q = q.Order("percent DESC NULLS LAST")
	default: // ending soonest first
		q = q.Order("end_at ASC NULLS LAST")
	}
	limit := clamp(atoi(c.Query("limit"), 20), 1, 100)
	offset := max(atoi(c.Query("offset"), 0), 0)
	var rows []models.Promotion
	if err := q.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	out := make([]PromotionView, len(rows))
	for i := range rows {
		out[i] = View(&rows[i], lang, false)
	}
	return c.JSON(fiber.Map{"lang": lang, "total": total, "limit": limit, "offset": offset, "promotions": out})
}

func (s *Server) promotion(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad id")
	}
	var p models.Promotion
	if err := s.DB.First(&p, id).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	}
	return c.JSON(fiber.Map{"lang": Lang(c), "promotion": View(&p, Lang(c), true)})
}

func (s *Server) crawlRuns(c fiber.Ctx) error {
	var runs []models.CrawlRun
	if err := s.DB.Order("id DESC").Limit(40).Find(&runs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"runs": runs})
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func clamp(n, lo, hi int) int { return min(max(n, lo), hi) }
