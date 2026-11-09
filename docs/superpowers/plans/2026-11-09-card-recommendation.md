# Card Recommendation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `GET /v1/recommend?merchant=&cards=&amount=` ranks a merchant's promotions by baht value, says which held card to use, and suggests cards worth applying for.

**Architecture:** Card / network / merchant catalog lives in embedded YAML (`app/catalog`), loaded in memory. `normalize` gains three extractors (card eligibility, merchants, baht-off) whose results are stored per promotion (`eligibility` JSONB column, `promotion_merchants` table). A pure `app/recommend` package does value math, eligibility checks and ranking; `app/api` wires it to HTTP.

**Tech Stack:** Go, Fiber v3, GORM + Postgres, goose migrations, `gopkg.in/yaml.v3`.

**Spec:** `docs/superpowers/specs/2026-11-09-card-recommendation-design.md`

**Deviations from spec (decided while planning — simpler, same behavior):**
- `networks` / `cards` / `merchants` are not DB tables: the embedded YAML is the single source, read at start. DB tables come when admin entry is built (spec non-goal).
- `promotion_eligibility` is a JSONB column `promotions.eligibility`, not a table: it is only ever read with its promotion. `promotion_merchants` *is* a table because recommend queries by merchant.
- `excluded_card_keys` sits once on the eligibility, not per rule.
- Points: no baht rate configured yet → points promotions go to `no_value` (spec: "ธนาคารที่ยังไม่ตั้งอัตรา → no_value"). No rate config is built.

## Global Constraints

- Every schema change = a new goose migration: `go run ./cmd/migrate create <name>`; never edit one that ran.
- Sources: robots.txt only, polite `fetch.Client`, never work around bot protection (KBank, Mastercard stay off).
- Wrong card recommended is worse than no promotion: unsure eligibility → `confidence: "low"`; low never appears in `consider_applying`.
- Never invent a baht value: no computable value → `no_value`.
- Default amounts when `amount` missing: fuel 1500, dining 1000, shopping 2000, everything else 1000; response says `amount_assumed: true`.
- Apply suggestion threshold: `extra_vs_best_held >= max(50, 0.2 × best_held_value)`, at most 3 cards, never from category `new-card`.
- Sort: `value_baht` desc → confidence high > medium > low → `end_at` soonest first (nil last).
- Every endpoint takes `lang` (existing `api.Lang`).
- `golangci-lint run ./...` (binary at `/Users/saharak.manoo/go/bin`) and `go test ./...` green before each commit. Commit messages carry no AI attribution.

## Review Focus

1. Short aliases matching inside other words ("PT" inside "PTT", "พีที" inside "พีทีที") → expect no match; pinned in Task 4 (`TestMerchants_ShortAliasNeedsBoundary`).
2. Card keys typed with different case/spaces (`TTB-So-Smart `) → expect same as `ttb-so-smart`; pinned in Task 6 (`TestParseCards_NormalizesAndDedups`).
3. Promotion with no end date and detail not yet fetched → must not be recommended (same rule as `/v1/promotions` active filter); pinned in Task 5 (`TestRecommend_SkipsUnknownDates`).
4. `amount` = `0`, negative, or `abc` → 400 with message, not a default; missing → default; pinned in Task 6 (`TestParseAmount`).
5. Holder has cards but none eligible → `best` empty, `consider_applying` still filled; same card sent twice → counted once; pinned in Task 5 (`TestRecommend_NoHeldCardEligible`).

---

### Task 1: Catalog package (networks, cards, merchants)

**Files:**
- Create: `app/catalog/catalog.go`, `app/catalog/cards.yaml`, `app/catalog/merchants.yaml`
- Test: `app/catalog/catalog_test.go`
- Modify: `app/models/labels.go` (add category `"fuel": {TH: "น้ำมัน", EN: "Fuel"}`), `go.mod` (`gopkg.in/yaml.v3`)

**Interfaces:**
- Produces:
  ```go
  type Network struct{ Key, NameTH, NameEN string; Tiers []string } // low → high
  type Card struct {
      Key, Bank, NameTH, NameEN string
      Aliases  []string
      Network, Tier, CoBrand, ApplyURL string
      Verified bool
  }
  type Merchant struct{ Key, NameTH, NameEN, Category string; Aliases []string }
  type Catalog struct{ Networks map[string]Network; Cards []Card; Merchants []Merchant }
  func Load() (*Catalog, error)                       // embedded YAML
  func Parse(cards, merchants []byte) (*Catalog, error)
  func (c *Catalog) Card(key string) (Card, bool)
  func (c *Catalog) Merchant(key string) (Merchant, bool)
  func (c *Catalog) ResolveMerchant(keyOrName string) (Merchant, bool) // key, or any alias/name, case-insensitive
  func (c *Catalog) MerchantsIn(category string) []Merchant
  func (c *Catalog) TierIndex(network, tier string) int // -1 when unknown
  func NormKey(s string) string // lower, trim
  ```
- YAML keys are snake_case of the field names (`name_th`, `apply_url`, `co_brand`).

- [ ] **Step 1: Write failing tests**
  - `TestLoad_EmbeddedCatalogValid`: `Load()` no error; `Card("ttb-so-smart")` found; `Merchant("bangchak").Category == "fuel"`.
  - `TestTierIndex`: `TierIndex("jcb","ultimate") > TierIndex("jcb","platinum")`; `TierIndex("visa","nope") == -1`; `TierIndex("nope","gold") == -1`.
  - `TestResolveMerchant`: `"บางจาก"`, `"Bangchak"`, `"BANGCHAK"`, `"bangchak"` → key `bangchak`; `"ไม่มีร้านนี้"` → false.
  - `TestParse_Rejects`: duplicate card key, unknown network, tier not in network, empty bank, duplicate merchant key → each returns error naming the key.
- [ ] **Step 2: Run** `go test ./app/catalog/` → FAIL (package missing)
- [ ] **Step 3: Implement** `catalog.go`. Networks are a Go var (not YAML):
  visa `classic, gold, platinum, signature, infinite`; mastercard `standard, gold, titanium, platinum, world, world-elite`; jcb `standard, gold, platinum, ultimate`; unionpay `standard, gold, platinum, diamond`; amex `green, gold, platinum`. Embed YAML with `//go:embed`.
- [ ] **Step 4: Write `cards.yaml`** from spec §10, every entry `verified: false`. Krungsri aliases must include these exact `CardType` strings (from DB):
  `บัตรเครดิต กรุงศรี คอร์ปอเรท การ์ด, บัตรเครดิต กรุงศรี ซิกเนเจอร์, บัตรเครดิต กรุงศรี ดอกเตอร์การ์ด, บัตรเครดิต กรุงศรี นาว แพลทินัม, บัตรเครดิต กรุงศรี วีซ่า/มาสเตอร์การ์ด, บัตรเครดิต กรุงศรี เจซีบี อัลติเมท, บัตรเครดิต กรุงศรี เจซีบี แพลทินัม, บัตรเครดิต กรุงศรี เลดี้ ไทเทเนี่ยม, บัตรเครดิต กรุงศรี เอ็กซ์คลูซีฟ ซิกเนเจอร์, บัตรเครดิต กรุงศรี แพลทินัม, บัตรเครดิต กรุงศรี แพลทินัม มาสเตอร์การ์ด, บัตรเครดิต กรุงศรี โฮมโปร คอร์ปอเรท การ์ด, บัตรเครดิต กรุงศรี ไพรเวท แบงก์กิ้ง, บัตรเครดิต ทีซีซี พริวิเลจ การ์ด, บัตรเครดิต สยาม ทาคาชิมายะ วีซ่า, บัตรเครดิต สยาม ทาคาชิมายะ เจซีบี, บัตรเครดิต สยาม ทาคาชิมายะ ไฟน์เนส, บัตรเครดิต สยาม ทาคาชิมายะ ไฟน์เนส - อินวิเทชั่น โอนลี่, บัตรเครดิต เอไอเอ วีซ่า, บัตรเครดิต เอไอเอ วีซ่า แพลทินัม, บัตรเครดิต โฮมโปร วีซ่า แพลทินัม`
  KTC aliases must include the exclusion names seen in text: `KTC CASH BACK`, `KTC - ROYAL ORCHID PLUS`, `KTC VISA CORPORATE`, `KTC GOVERNMENT SERVICES`, `KTC - THAI RED CROSS NATIONAL BLOOD CENTRE`. ttb: `ttb-disney` with aliases `ttb Disney, ทีทีบี ดิสนีย์`. Cards whose network/tier is unknown get the best guess from the name and stay `verified: false`.
- [ ] **Step 5: Write `merchants.yaml`**. Fuel (`category: fuel`): bangchak (`บางจาก, Bangchak, BCP`), ptt-station (`ปตท., PTT Station, PTT, พีทีที`), shell (`เชลล์, Shell`), caltex (`คาลเท็กซ์, Caltex`), pt (`พีที, PT`), susco (`ซัสโก้, Susco`). Others: shopee, lazada, central (`เซ็นทรัล, Central`), the-mall (`เดอะมอลล์, The Mall`), seven-eleven (`เซเว่น, 7-Eleven, 7-11`), big-c (`บิ๊กซี, Big C`), lotuss (`โลตัส, Lotus's`), homepro (`โฮมโปร, HomePro`), starbucks, mcdonalds (`แมคโดนัลด์, McDonald's, McDonald’s`), grab, agoda, major-cineplex (`เมเจอร์ ซีนีเพล็กซ์, Major Cineplex`).
- [ ] **Step 6: Run** `go test ./app/catalog/` → PASS
- [ ] **Step 7: Commit** `feat(catalog): card, network and merchant catalog`

### Task 2: Schema — eligibility, amount off, promotion merchants

**Files:**
- Modify: `app/models/promotion.go`
- Create: `app/models/eligibility.go`, migrations via `go run ./cmd/migrate create add_eligibility_and_amount_off_to_promotions` and `... create create_promotion_merchants`

**Interfaces:**
- Produces:
  ```go
  const (ScopeBankAll = "bank_all"; ScopeCards = "cards"; ScopeNetwork = "network")
  const (ConfHigh = "high"; ConfMedium = "medium"; ConfLow = "low")
  type EligibilityRule struct {
      Scope    string   `json:"scope"`
      CardKeys []string `json:"card_keys,omitempty"`
      Network  string   `json:"network,omitempty"`
      MinTier  string   `json:"min_tier,omitempty"`
      Issuer   string   `json:"issuer,omitempty"` // "" = any bank
  }
  type Eligibility struct {
      Rules      []EligibilityRule `json:"rules"`
      Excluded   []string          `json:"excluded,omitempty"`
      Confidence string            `json:"confidence"`
      Unmatched  []string          `json:"unmatched,omitempty"`
  }
  // on Promotion:
  AmountOff   *int        `json:"amount_off"`
  Eligibility Eligibility `gorm:"type:jsonb;serializer:json" json:"eligibility"`
  type PromotionMerchant struct {
      PromotionID uint   `gorm:"primaryKey;autoIncrement:false"`
      MerchantKey string `gorm:"primaryKey;size:50;index"`
      Match       string `gorm:"size:10;not null"` // "merchant" | "category"
  }
  ```
- [ ] **Step 1:** Add types and fields; fill the two generated migrations (up: `AddColumn` guarded by `HasColumn` / `AutoMigrate(&models.PromotionMerchant{})`; down: `DropColumn` / `DropTable`).
- [ ] **Step 2: Verify on a scratch DB:** `createdb kardee_mtest && DATABASE_URL=postgres://localhost:5432/kardee_mtest?sslmode=disable go run ./cmd/migrate up`, then `down` ×2, then `up` → all succeed; `psql ... -c '\d promotions'` shows `eligibility jsonb`, `amount_off`; `\d promotion_merchants` has PK `(promotion_id, merchant_key)`. `dropdb kardee_mtest`.
- [ ] **Step 3:** `go run ./cmd/migrate up` on dev DB; `select count(*) from promotions` unchanged.
- [ ] **Step 4: Commit** `feat(models): eligibility, amount_off and promotion_merchants`

### Task 3: Card eligibility extraction

**Files:**
- Create: `app/normalize/eligibility.go`, `app/normalize/eligibility_test.go`

**Interfaces:**
- Consumes: Task 1 `Catalog`, Task 2 `models.Eligibility`
- Produces: `func ExtractEligibility(cat *catalog.Catalog, bank string, cardTypes []string, text string) models.Eligibility`
  (`cardTypes` = `strings.Split(p.Cards, "|")` minus empties; `text` = title + description + detail TH + detail EN)

Rules, in order:
1. `bank` is a network key (`visa`, `jcb`) → one `network` rule, `Issuer ""`, `MinTier` = lowest tier named in text for that network (or "" = any), `medium`.
2. `cardTypes` non-empty → each matched by alias (normalized: upper-case, `-`→space, spaces collapsed, exact) → `cards` rule, `high`; unmatched names → `Unmatched`.
3. Text: network-wide phrase `<NETWORK> <TIER> (หรือ|or|/|,) <NETWORK> <TIER> ... ที่ออกในประเทศไทย|issued in Thailand` → `network` rule, `Issuer ""`, `MinTier` = lowest named tier. `<BANK> <NETWORK> ทุกประเภท` (e.g. `KTC JCB ทุกประเภท`) → `network` rule `Issuer=bank`, no tier. Card aliases of this bank appearing after `เฉพาะ` (same line) → `cards` rule. Card aliases appearing after `ยกเว้น` / `สงวนสิทธิ์ยกเว้น` / `except` (same line) → `Excluded`. Longest alias wins (sort by length desc, mark matched spans). Thai aliases match as substrings; aliases made only of ASCII letters/digits need non-letter/digit on both sides.
4. No rule found → `bank_all`; confidence `medium` if text has `บัตรเครดิต <bank name or bank alias>` / `ทุกประเภท` / `ทุกใบ`, else `low`.
   Bank aliases: ktc `KTC, เคทีซี`; ttb `ttb, ทีทีบี`; krungsri `กรุงศรี, Krungsri`; kbank `กสิกร, KBank`.

- [ ] **Step 1: Write failing tests** (fixtures are real texts from DB; paste them as Go raw strings):
  - `TestEligibility_CrownDishJCBAnyBank`: promo 28 text → contains rule `{Scope: network, Network: jcb, MinTier: platinum, Issuer: ""}` **and** rule `{network, jcb, Issuer: ktc}` (from "KTC JCB ทุกประเภท").
  - `TestEligibility_KTCExclusions`: promo 22 text → rules `[bank_all]`, `Excluded` ⊇ keys of KTC CASH BACK, ROYAL ORCHID PLUS, VISA CORPORATE, GOVERNMENT SERVICES, THAI RED CROSS; confidence `medium`.
  - `TestEligibility_TTBDisneyOnly`: title `ดีลเอ็กซ์คลูซีฟ เฉพาะบัตร ทีทีบี ดิสนีย์ เท่านั้น` → `cards [ttb-disney]`.
  - `TestEligibility_KrungsriCardTypes`: two known names + `บัตรเครดิต ไม่มีจริง` → `cards` with the two keys, `high`, `Unmatched == ["บัตรเครดิต ไม่มีจริง"]`.
  - `TestEligibility_GenericTTB`: `เป็นเจ้าของ iPhone ด้วยบัตรเครดิต ttb` → `bank_all`, `medium`.
  - `TestEligibility_NothingSaid`: `""` → `bank_all`, `low`.
  - `TestEligibility_VisaSource`: bank `visa`, text `สำหรับบัตร Visa Signature และ Visa Infinite` → `{network, visa, MinTier: signature, Issuer: ""}`.
- [ ] **Step 2: Run** `go test ./app/normalize/ -run Eligibility` → FAIL
- [ ] **Step 3: Implement** per the rules above.
- [ ] **Step 4: Run** → PASS
- [ ] **Step 5: Commit** `feat(normalize): extract card eligibility`

### Task 4: Merchants and baht-off extraction

**Files:**
- Create: `app/normalize/merchants.go`, `app/normalize/merchants_test.go`
- Modify: `app/normalize/normalize.go` (`Conditions.AmountOff *int`, set in `Extract`, copied in `Apply`), `app/normalize/normalize_test.go`

**Interfaces:**
- Produces:
  ```go
  type MerchantMatch struct{ Key, Match string } // Match: "merchant" | "category"
  func ExtractMerchants(cat *catalog.Catalog, text string) []MerchantMatch
  ```
  Alias matching uses the same matcher as Task 3 (share it: `alias.go` with `type matcher` if both need it). No named merchant + text has `น้ำมัน|fuel` and `ปั๊ม|สถานีบริการ|station` → every `MerchantsIn("fuel")` with `Match: "category"`.
- `AmountOff`: patterns `ลด(?:ทันที)?\s*NUM\s*บาท`, `ส่วนลด\s*NUM\s*บาท`, `(?i)(?:THB|฿)\s*NUM\s*off`, `(?i)NUM\s*(?:THB|baht)\s*off`; highest wins. Not set when the number is followed by `/เดือน` or preceded by `สูงสุด` (that is a cap, already `MaxBenefit`).

- [ ] **Step 1: Write failing tests**
  - `TestMerchants_Bangchak`: `เติมน้ำมันสุดคุ้มที่ปั๊มบางจาก` → `[{bangchak, merchant}]`.
  - `TestMerchants_GenericFuelStation`: `เติมน้ำมันครบทุก 700 บาท/สลิป รับเครดิตเงินคืน 5% ที่ปั๊มที่ร่วมรายการ` → all 6 fuel keys, each `category`.
  - `TestMerchants_ShortAliasNeedsBoundary`: `PTT Station` → contains `ptt-station`, not `pt`; `SPTX` → none; `พีทีที` → `ptt-station`, not `pt`.
  - `TestMerchants_Several`: `11.11 Shopee และ Lazada` → `shopee`, `lazada`.
  - `TestExtract_AmountOff`: `รับส่วนลดทันที 200 บาท เมื่อใช้จ่ายครบ 2,000 บาท` → `AmountOff 200`, `MinSpend 2000`; `เครดิตเงินคืนสูงสุด 500 บาท` → `AmountOff nil`.
- [ ] **Step 2: Run** `go test ./app/normalize/` → FAIL
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run** → PASS
- [ ] **Step 5: Commit** `feat(normalize): extract merchants and baht-off discounts`

### Task 5: Recommend package (value, eligibility, ranking)

**Files:**
- Create: `app/recommend/recommend.go`, `app/recommend/recommend_test.go`

**Interfaces:**
- Consumes: Tasks 1–2
- Produces:
  ```go
  type Candidate struct{ Promotion *models.Promotion; Match string }
  type Input struct {
      Cards  []string // normalized keys, user order, deduped
      Amount int
      Now    time.Time
  }
  type Item struct {
      Promotion     *models.Promotion
      ValueBaht     float64
      UseCard       *catalog.Card  // nil when no cards given
      EligibleCards []catalog.Card // catalog cards that can use it
      Confidence    string
      MerchantMatch string
      NeedsMinSpend *int // set when Amount < MinSpend
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
  func DefaultAmount(category string) int
  func Value(p *models.Promotion, amount int) (baht float64, ok bool, needsMin *int)
  func Eligible(cat *catalog.Catalog, c catalog.Card, p *models.Promotion) bool
  func Recommend(cat *catalog.Catalog, in Input, cands []Candidate) Result
  ```
- `Value`: `cashback`/`discount` with `Percent` → `amount×pct/100` capped by `MaxBenefit`; else `AmountOff` → that; `amount < MinSpend` → `0, true, &MinSpend`; anything else → `ok=false`.
- `Eligible` implements spec §5; empty `Rules` = `bank_all`.
- Candidate is dropped unless `p.Active(in.Now)` **and** (`EndAt != nil || DetailFetchedAt != nil`) — same rule as the `/v1/promotions` active filter.
- With cards: Item kept only if some held card is eligible; `UseCard` = first eligible in user order. Without cards: every valued candidate, `UseCard nil`.
- `ConsiderApplying` only when cards given: candidates no held card can use, `ok`, confidence ≠ low, category ≠ `new-card`; card = first eligible catalog card with `ApplyURL != ""`, else first eligible; best promotion per card; threshold from Global Constraints; top 3 by value.

- [ ] **Step 1: Write failing tests** (build promotions in Go):
  - `TestValue`: 5% cap 100 at 1500 → 75; at 3000 → 100; min 2000 at 1500 → `0, ok, needsMin=2000`; `AmountOff 200` → 200; `installment` → `ok=false`.
  - `TestEligible`: `bank_all` ktc → ktc card yes, ttb card no; `cards [x]` → only x; `network jcb MinTier platinum Issuer ""` → krungsri JCB ultimate yes, JCB gold no, Visa no; `Issuer ktc` → krungsri JCB no; `Excluded [x]` + `bank_all` → x no.
  - `TestRecommend_RanksByValueThenConfidenceThenEnd`: three items with values 100/100/50, confidence high/medium, end dates → expected order.
  - `TestRecommend_UseFirstHeldEligibleCard`: cards `[b, a]` both eligible → `UseCard == b`.
  - `TestRecommend_ConsiderApplyingThreshold`: best held 100; other card 140 → none (40 < 50); 160 → suggested with `ExtraVsBestHeld 60`; low-confidence 500 → not suggested; `new-card` 500 → not suggested; 4 qualifying cards → 3 returned.
  - `TestRecommend_SkipsUnknownDates`: `EndAt nil`, `DetailFetchedAt nil` → absent from all lists; expired → absent.
  - `TestRecommend_NoHeldCardEligible`: held card ineligible for all → `Best` empty, `ConsiderApplying` non-empty.
  - `TestRecommend_NoValueSeparated`: `gift` promo eligible → in `NoValue`, not in `Best`.
  - `TestRecommend_UnknownCards`: `Cards ["nope"]` → `UnknownCards ["nope"]`.
  - `TestDefaultAmount`: fuel 1500, dining 1000, shopping 2000, travel 1000.
- [ ] **Step 2: Run** `go test ./app/recommend/` → FAIL
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run** → PASS
- [ ] **Step 5: Commit** `feat(recommend): value, eligibility and ranking`

### Task 6: Wire normalize into crawler, renormalize command, API

**Files:**
- Create: `app/normalize/promotion.go`, `app/api/recommend.go`, `app/api/recommend_test.go`
- Modify: `app/crawler/crawler.go:21-27,112-117`, `cmd/jobs/main.go`, `cmd/api/main.go`, `app/api/api.go` (`Server` gets `Catalog *catalog.Catalog`; routes), `README.md` (endpoints + `jobs -renormalize`)

**Interfaces:**
- Produces:
  ```go
  // normalize: Extract+Apply, eligibility, merchants; sets Category "fuel" when a fuel
  // merchant matched and Category is "auto" or "other". Returns rows to store.
  func Promotion(cat *catalog.Catalog, p *models.Promotion) []models.PromotionMerchant
  // crawler: saves p and replaces its promotion_merchants rows in one transaction.
  func Save(db *gorm.DB, p *models.Promotion, ms []models.PromotionMerchant) error
  // Crawler gains field: Catalog *catalog.Catalog
  // api helpers:
  func parseCards(s string) []string            // split ",", NormKey, drop empty, dedup, keep order
  func parseAmount(s string) (n int, given bool, err error) // "" → 0,false,nil; "<=0"/non-int → error
  ```
- Routes: `GET /v1/merchants?q=&category=`, `GET /v1/cards?bank=&q=`, `GET /v1/networks`, `GET /v1/recommend`.
- `/v1/recommend`: `merchant` missing → 400 `"merchant is required"`; unresolvable → 404 `"unknown merchant"`; bad amount → 400 `"amount must be a positive whole number of baht"`. Loads promotions `JOIN promotion_merchants ON promotion_id = id WHERE merchant_key = ? AND NOT gone`; response shape exactly spec §9 (`promotion` = existing `View(p, lang, false)`; card objects `{key, name, bank, network, tier, apply_url}` localized; `amount_assumed`; `unknown_cards`).
- `jobs -renormalize`: `FindInBatches(500)` over all promotions → `normalize.Promotion` → `crawler.Save`; logs counts: total, with eligibility rules ≠ bank_all-low, with merchants, with value fields.

- [ ] **Step 1: Write failing tests**
  - `TestParseCards_NormalizesAndDedups`: `" TTB-So-Smart ,ttb-so-smart,,ktc-x"` → `["ttb-so-smart","ktc-x"]`.
  - `TestParseAmount`: `""`→`0,false,nil`; `"1500"`→`1500,true,nil`; `"0"`, `"-5"`, `"abc"`, `"1.5"` → error.
  - `TestPromotion_SetsFuelCategory` (normalize): promo category `auto`, title `เติมน้ำมันที่ปั๊มบางจาก` → category `fuel`, rows `[{bangchak, merchant}]`.
- [ ] **Step 2: Run** `go test ./app/...` → FAIL
- [ ] **Step 3: Implement**; crawler calls `normalize.Promotion` + `Save` instead of the old `Apply` + `DB.Save`.
- [ ] **Step 4: Run** `go test ./...` → PASS; lint clean.
- [ ] **Step 5: Verify on dev DB** (after the backlog crawl finished): `go run ./cmd/jobs -renormalize`; then
  `curl -s 'localhost:8090/v1/recommend?merchant=บางจาก&cards=krungsri-platinum&amount=1500' | jq '.best[0], .consider_applying, .amount_assumed'` → JSON with the spec §9 keys; `curl -s 'localhost:8090/v1/recommend?merchant=bangchak&amount=abc'` → 400.
  Record before/after counts (benefit_type ≠ other, has percent or amount_off, has merchants, eligibility confidence by level) in the commit message body.
- [ ] **Step 6: Commit** `feat: recommend API and renormalize job`

### Task 7: Visa and JCB sources

**Files:**
- Create: `app/sources/visa.go`, `app/sources/jcb.go`, tests in `app/sources/sources_test.go` with saved HTML/JSON fixtures under `app/sources/testdata/`
- Modify: `app/sources/source.go` (`All`), `app/models/labels.go` (`Banks`: `visa`, `jcb`)

**Interfaces:**
- Consumes: existing `Source` interface; Task 3 rule 1 handles eligibility for these banks.

- [ ] **Step 1: Recon (no code):** with the polite client UA, find how each site lists Thai offers without JS: Visa — sitemap `https://www.visa.co.th/th_TH/sitemap.xml` entries under `/pay-with-visa/visa-offers-and-perks/` (static HTML per offer); JCB — `https://www.specialoffers.jcb/th/offers/d/southeast-and-south-asia/thailand/` (find the data request the page makes; usable only if it's a plain public GET allowed by robots). **Stop condition:** if a site needs a challenge, login, or a disallowed path, do not build that source — write the finding in the commit message and README.
- [ ] **Step 2: Save one list + one detail fixture per source; write failing tests** `TestVisaListing`, `TestVisaDetail`, `TestJCBListing` asserting ExternalID, TitleTH, dates and a non-empty TextTH from the fixtures.
- [ ] **Step 3: Implement**, run `go test ./app/sources/` → PASS
- [ ] **Step 4: Live run** `go run ./cmd/jobs -once -bank visa` (and `jcb`) → rows with `bank='visa'` and eligibility `network`.
- [ ] **Step 5: Commit** `feat(sources): Visa and JCB Thailand offers`

### Task 8: Verify the card catalog and report gaps

**Files:**
- Modify: `app/catalog/cards.yaml`

- [ ] **Step 1:** For KTC, ttb, Krungsri: fetch each bank's public credit card list page (robots allowed, no challenge); for each catalog card confirm network + tier + apply URL; set `verified: true` only when the page states them. KBank stays unverified (site blocked).
- [ ] **Step 2:** `select distinct jsonb_array_elements_text(eligibility->'unmatched') from promotions` → add missing cards/aliases.
- [ ] **Step 3 (value coverage, spec §7.3):** sample 30 active `fuel` + `dining` promotions where `percent IS NULL AND amount_off IS NULL`; for each wording that states a value but isn't caught, add a `TestExtract_*` case with that text, then the pattern in `normalize.go`. Done when the sample's caught share is reported before/after.
- [ ] **Step 4:** `go test ./...`, `go run ./cmd/jobs -renormalize`, commit `chore(catalog): verify cards against bank pages`.
- [ ] **Step 5:** Report to the user: verified vs unverified cards, unmatched names left, value coverage numbers.
