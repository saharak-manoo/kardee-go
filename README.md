# Kardee (การ์ดดี)

Credit card promotions from Thai banks, read from each bank's public website and
turned into short, comparable summaries (benefit, percent, minimum spend, cap,
registration, end date) in Thai and English. Same layout as etrackings: an API
process and a jobs process.

```
cmd/api     REST API (later: LINE webhook)
cmd/jobs    crawls the banks on a schedule
cmd/migrate schema migrations (goose, app/database/migrations)
app/
  sources/    one file per bank: list + detail parsing
  normalize/  pulls numbers out of the text, writes TH/EN summaries
  crawler/    list -> save -> read changed detail pages -> normalize -> mark gone
  fetch/      polite HTTP client (one request at a time per host, pause, honest UA)
  api/        Fiber handlers, ?lang=th|en
  models/     Promotion, CrawlRun, TH/EN labels
```

## Banks

| Bank | Source | Notes |
|---|---|---|
| KTC | ktc.co.th sitemap (lastmod) + page RSC data | TH + EN. Only pages changed in `KTC_MAX_AGE_DAYS`. ~2,750 pages, so detail pages fill in over several runs (`CRAWL_DETAIL_LIMIT` per run). |
| ttb | ttbbank.com `__NEXT_DATA__`, paged per sub-category | TH + EN (EN when ttb wrote it). |
| Krungsri | krungsricard.com `var promotionData` (one page, ~560 items) | TH only: robots.txt disallows `/en/promotion`. Detail pages are landing pages; full conditions are often in PDFs we don't read. Needs TLS RSA key exchange (see `fetch.legacyTLSHosts`). |
| KBank | — | Off. kasikornbank.com answers 403 (Akamai) to automated requests. We don't work around bot protection. |

## Languages

Every text column has `_th` and `_en`. The API answers in `?lang=` (or
`Accept-Language`), falls back to Thai when the bank has no English, and says which
language the text really is in `text_lang`. Summaries are generated in both languages
from the extracted numbers, so even Thai-only promotions get an English summary.
Category, bank and benefit names come from `models/labels.go`.

## Run

```bash
cp .env.example .env          # DATABASE_URL etc.
createdb kardee_development
go run ./cmd/jobs -once                 # crawl every bank once
go run ./cmd/jobs -once -bank ttb       # one bank
go run ./cmd/jobs                       # on CRAWL_CRON (Thai time)
go run ./cmd/jobs -renormalize          # re-extract cards/merchants/values after catalog or extractor changes
go run ./cmd/api                        # :8090
go run ./cmd/migrate status             # which migrations ran (api/jobs migrate on start)
go run ./cmd/migrate create add_merchants  # new migration file; never edit one that ran
go run ./cmd/migrate down               # undo the last one
```

## API

```
GET /v1/promotions?lang=en&bank=ktc,ttb&category=dining&benefit=cashback&q=shopee&active=true&sort=ending|newest|percent&limit=20&offset=0
GET /v1/promotions/:id?lang=th      # adds the full conditions text
GET /v1/banks?lang=en
GET /v1/categories?lang=en          # categories + benefit types
GET /v1/crawl-runs                  # last runs per bank, with errors
GET /v1/merchants?q=บางจาก&category=fuel
GET /v1/cards?bank=ttb&q=smart       # card catalog (for the "my cards" picker)
GET /v1/networks
GET /v1/recommend?merchant=bangchak&cards=ttb-so-smart,kbank-the-passion&amount=1500
```

`active=true` (default) hides promotions whose dates we don't know yet.

`/v1/recommend` ranks the merchant's promotions by baht value at `amount` (default per category,
`amount_assumed: true`), says which held card to use (`best[].use_card`), lists cards worth
applying for (`consider_applying`), and keeps promotions with no computable value apart (`no_value`).
Cards, networks and merchants live in `app/catalog/*.yaml`; after editing them run `jobs -renormalize`.

## Crawling rules

- Only pages robots.txt allows; one request at a time per bank with `CRAWL_DELAY_SECONDS` between.
- A 403/429 stops that bank's run (`fetch.ErrBlocked`); no retries around it.
- Detail pages are read again only when the bank's version (lastmod / modified date / content hash) changes.

## Users

Not built yet. Plan: own `users` table, not shared with etrackings. The LINE bot goes
under the same LINE provider as etrackings, so a LINE userId is the same in both and
accounts can be linked later without sharing tables.
