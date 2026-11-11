package sources

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/saharak/kardee-go/app/fetch"
)

// AEON lists its promotions in a sitemap with lastmod. Each page is plain HTML: a
// "pro-card-details" block with the title, the period ("ระยะเวลา :"), the participating
// cards ("บัตรที่ร่วมรายการ :") and the conditions.
type AEON struct{}

const aeonBase = "https://www.aeon.co.th"

func (a *AEON) Bank() string { return "aeon" }

func (a *AEON) List(ctx context.Context, c *fetch.Client) ([]Listing, error) {
	b, err := c.Get(ctx, aeonBase+"/sitemaps/sitemap-promotions.xml")
	if err != nil {
		return nil, err
	}
	return aeonListings(b)
}

var aeonPromoRe = regexp.MustCompile(`^/aeon/promotions/([^/]+)/?$`)

func aeonListings(b []byte) ([]Listing, error) {
	var set urlSet
	if err := xml.Unmarshal(b, &set); err != nil {
		return nil, fmt.Errorf("aeon sitemap: %w", err)
	}
	var out []Listing
	for _, u := range set.URLs {
		pu, err := url.Parse(strings.TrimSpace(u.Loc))
		if err != nil {
			continue
		}
		m := aeonPromoRe.FindStringSubmatch(pu.Path)
		if m == nil {
			continue
		}
		out = append(out, Listing{ExternalID: m[1], URLTH: strings.TrimSpace(u.Loc), Version: strings.TrimSpace(u.LastMod), Category: mapCategory(m[1])})
	}
	return out, nil
}

func (a *AEON) Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error) {
	b, err := c.Get(ctx, l.URLTH)
	if err != nil {
		return Detail{}, err
	}
	d := parseAEONPage(string(b))
	if d.TitleTH == "" {
		return Detail{}, fmt.Errorf("%s: no promotion block", l.URLTH)
	}
	return d, nil
}

var (
	aeonImgRe    = regexp.MustCompile(`<img[^>]+src="(/contentAsset/raw-data/[^"?]+)`)
	aeonPeriodRe = regexp.MustCompile(`ระยะเวลา\s*:\s*([^\n]+)`)
	aeonCardsRe  = regexp.MustCompile(`บัตรที่ร่วมรายการ\s*:\s*([^\n]+)`)
	aeonSplitRe  = regexp.MustCompile(`\s*,\s*|\s+และ\s*`)
)

const (
	aeonBlock     = `<div class="pro-card-details__heading"`
	aeonApplyEnd  = "日本人のお客様はこちらからお申込みください"
	aeonLegalLine = "ใช้เท่าที่จำเป็นและชำระคืน"
)

func parseAEONPage(page string) Detail {
	var d Detail
	i := strings.Index(page, aeonBlock)
	if i < 0 {
		return d
	}
	if m := aeonImgRe.FindStringSubmatch(page[max(0, i-1500):i]); m != nil {
		d.ImageURL = aeonBase + m[1]
	}
	lines := strings.Split(HTMLText(page[i:]), "\n")
	d.TitleTH = lines[0]
	// The "apply via the app" box sits between the summary and the offer, shown twice:
	// drop everything from "สมัครบัตร" to its last closing line.
	lastApply := -1
	for n, l := range lines {
		if l == aeonApplyEnd {
			lastApply = n
		}
	}
	var keep []string
	skipping := false
	for n, l := range lines[1:] {
		n++
		if strings.HasPrefix(l, aeonLegalLine) {
			break // the legal line ends the offer
		}
		if l == "สมัครบัตร" && n < lastApply {
			skipping = true
		}
		if !skipping {
			keep = append(keep, l)
		}
		if n == lastApply {
			skipping = false
		}
	}
	d.TextTH = strings.Join(keep, "\n")
	if m := aeonPeriodRe.FindStringSubmatch(d.TextTH); m != nil {
		d.StartAt, d.EndAt = ParseThaiPeriod(m[1])
	}
	if m := aeonCardsRe.FindStringSubmatch(d.TextTH); m != nil {
		d.Cards = splitAEONCards(m[1])
	}
	return d
}

// splitAEONCards splits "A, B และ C" into names, keeping commas inside parentheses
// ("ทุกประเภท (ยกเว้น X, Y)") with their entry. A trailing "ทุกประเภท" on a named card
// is dropped ("บัตรอิออนคลาสสิคทุกประเภท" = every kind of that card).
func splitAEONCards(s string) []string {
	var parts []string
	depth, start := 0, 0
	cut := func(end int) {
		for _, p := range aeonSplitRe.Split(s[start:end], -1) {
			if p = strings.TrimSpace(p); p != "" {
				parts = append(parts, p)
			}
		}
	}
	for i, r := range s {
		switch r {
		case '(':
			if depth == 0 {
				cut(i)
				start = i
			}
			depth++
		case ')':
			if depth--; depth == 0 {
				// glue the bracket to the entry before it
				if n := len(parts); n > 0 {
					parts[n-1] += " " + strings.TrimSpace(s[start:i+1])
				} else {
					parts = append(parts, strings.TrimSpace(s[start:i+1]))
				}
				start = i + 1
			}
		}
	}
	cut(len(s))
	out := parts[:0]
	for _, p := range parts {
		if !strings.Contains(p, "(") {
			p = strings.TrimSpace(strings.TrimSuffix(p, "ทุกประเภท"))
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
