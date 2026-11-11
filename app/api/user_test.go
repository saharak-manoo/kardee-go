package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/saharak/kardee-go/app/auth"
	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/database"
	"github.com/saharak/kardee-go/app/models"
)

// fakeLINE accepts "tok-<subject>" as a valid ID token.
type fakeLINE struct{}

func (fakeLINE) Verify(_ context.Context, token string) (auth.Identity, error) {
	sub, ok := strings.CutPrefix(token, "tok-")
	if !ok {
		return auth.Identity{}, auth.ErrInvalidToken
	}
	return auth.Identity{Provider: auth.ProviderLINE, Subject: sub, Name: "Name " + sub}, nil
}

// testServer needs TEST_DATABASE_URL (a database it may wipe); skipped without it.
func testServer(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := database.Open(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("TRUNCATE affiliate_clicks, search_logs, sessions, user_cards, user_identities, users, promotion_merchants, promotions RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	(&Server{DB: db, Catalog: cat, Verifier: fakeLINE{}}).Routes(app)
	return app, db
}

func call(t *testing.T, app *fiber.App, method, path, token, body string, headers ...string) (int, map[string]any, http.Header) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out, resp.Header
}

func login(t *testing.T, app *fiber.App, sub string) string {
	t.Helper()
	code, out, _ := call(t, app, "POST", "/v1/auth/line", "", `{"id_token":"tok-`+sub+`"}`)
	if code != 200 {
		t.Fatalf("login %s: %d %v", sub, code, out)
	}
	return out["token"].(string)
}

func cardKeys(out map[string]any) []string {
	var keys []string
	for _, c := range out["cards"].([]any) {
		keys = append(keys, c.(map[string]any)["key"].(string))
	}
	return keys
}

func TestAuthLine_SameLINEUserIsOneUser(t *testing.T) {
	app, db := testServer(t)
	login(t, app, "U1")
	login(t, app, "U1")
	var users, sessions int64
	db.Model(&models.User{}).Count(&users)
	db.Model(&models.Session{}).Count(&sessions)
	if users != 1 || sessions != 2 {
		t.Errorf("users %d sessions %d, want 1 and 2", users, sessions)
	}
	if code, _, _ := call(t, app, "POST", "/v1/auth/line", "", `{"id_token":"forged"}`); code != 401 {
		t.Errorf("bad token → %d, want 401", code)
	}
}

func TestMeCards_NeedsBearerNotHeaderID(t *testing.T) {
	app, _ := testServer(t)
	login(t, app, "U1")
	if code, _, _ := call(t, app, "GET", "/v1/me/cards", "", "", "X-Line-User-ID", "U1"); code != 401 {
		t.Errorf("X-Line-User-ID alone → %d, want 401", code)
	}
	if code, _, _ := call(t, app, "GET", "/v1/me/cards", "not-a-token", ""); code != 401 {
		t.Errorf("bad bearer → %d, want 401", code)
	}
}

func TestMeCards_KeepsUserOrder(t *testing.T) {
	app, _ := testServer(t)
	tok := login(t, app, "U1")
	body := `{"cards":["ttb-so-smart","KTC-JCB-ULTIMATE","krungsri-platinum","ttb-so-smart"]}`
	if code, out, _ := call(t, app, "PUT", "/v1/me/cards", tok, body); code != 200 {
		t.Fatalf("put → %d %v", code, out)
	}
	_, out, _ := call(t, app, "GET", "/v1/me/cards", tok, "")
	if got := strings.Join(cardKeys(out), ","); got != "ttb-so-smart,ktc-jcb-ultimate,krungsri-platinum" {
		t.Errorf("order = %s", got)
	}
	if code, out, _ := call(t, app, "PUT", "/v1/me/cards", tok, `{"cards":["nope"]}`); code != 400 {
		t.Errorf("unknown card → %d %v, want 400", code, out)
	}
}

func TestMeCards_ExpiredSession(t *testing.T) {
	app, db := testServer(t)
	tok := login(t, app, "U1")
	db.Model(&models.Session{}).Where("token_hash = ?", auth.HashToken(tok)).Update("expires_at", time.Now().Add(-time.Minute))
	if code, _, _ := call(t, app, "GET", "/v1/me/cards", tok, ""); code != 401 {
		t.Errorf("expired → %d, want 401", code)
	}
}

func TestDeleteMe_RemovesAccount(t *testing.T) {
	app, db := testServer(t)
	tok := login(t, app, "U1")
	call(t, app, "PUT", "/v1/me/cards", tok, `{"cards":["ttb-so-smart"]}`)
	if code, _, _ := call(t, app, "DELETE", "/v1/me", tok, ""); code != 204 {
		t.Fatalf("delete → %d", code)
	}
	if code, _, _ := call(t, app, "GET", "/v1/me/cards", tok, ""); code != 401 {
		t.Errorf("token after delete → %d, want 401", code)
	}
	var cards, identities int64
	db.Model(&models.UserCard{}).Count(&cards)
	db.Model(&models.UserIdentity{}).Count(&identities)
	if cards != 0 || identities != 0 {
		t.Errorf("left %d cards, %d identities", cards, identities)
	}
	login(t, app, "U1") // signing in again starts a fresh account
	var live int64
	db.Model(&models.User{}).Count(&live)
	if live != 1 {
		t.Errorf("live users %d, want 1", live)
	}
}

func TestRecommend_UsesSavedCardsAndLogsSearch(t *testing.T) {
	app, db := testServer(t)
	tok := login(t, app, "U1")
	call(t, app, "PUT", "/v1/me/cards", tok, `{"cards":["krungsri-platinum"]}`)
	end := time.Now().Add(24 * time.Hour)
	pct, maxB := 5.0, 100
	p := models.Promotion{Bank: "krungsri", ExternalID: "x", TitleTH: "บางจาก 5%", BenefitType: models.BenefitCashback,
		Percent: &pct, MaxBenefit: &maxB, EndAt: &end,
		Eligibility: models.Eligibility{Rules: []models.EligibilityRule{{Scope: models.ScopeBankAll}}, Confidence: models.ConfHigh}}
	db.Create(&p)
	db.Create(&models.PromotionMerchant{PromotionID: p.ID, MerchantKey: "bangchak", Match: models.MatchMerchant})

	code, out, _ := call(t, app, "GET", "/v1/recommend?merchant=bangchak&amount=1500", tok, "", "X-Kardee-Channel", "line")
	if code != 200 {
		t.Fatalf("recommend → %d %v", code, out)
	}
	best := out["best"].([]any)
	if len(best) != 1 || best[0].(map[string]any)["use_card"].(map[string]any)["key"] != "krungsri-platinum" {
		t.Errorf("best = %v", best)
	}
	var log models.SearchLog
	if err := db.First(&log).Error; err != nil {
		t.Fatal(err)
	}
	if log.UserID == nil || log.Channel != "line" || log.MerchantKey != "bangchak" || log.Amount != 1500 || log.CardsCount != 1 || log.ResultsCount != 1 {
		t.Errorf("log = %+v", log)
	}
}

func TestGoApply_RedirectsOnlyToCatalogAndLogs(t *testing.T) {
	app, db := testServer(t)
	code, _, h := call(t, app, "GET", "/v1/go/apply?card=ktc-jcb-ultimate&promotion=7", "", "")
	if code != 302 || h.Get("Location") != "https://www.ktc.co.th/credit-card" {
		t.Errorf("redirect %d → %q", code, h.Get("Location"))
	}
	var c models.AffiliateClick
	if err := db.First(&c).Error; err != nil || c.CardKey != "ktc-jcb-ultimate" || c.PromotionID == nil || *c.PromotionID != 7 || c.Channel != "web" {
		t.Errorf("click = %+v, %v", c, err)
	}
	if code, _, _ := call(t, app, "GET", "/v1/go/apply?card=nope", "", ""); code != 404 {
		t.Errorf("unknown card → %d, want 404", code)
	}
}
