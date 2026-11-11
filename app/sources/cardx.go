package sources

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/fetch"
)

// CardX (SCB's card company) lists every promotion page in its sitemap, Thai under
// /th-TH/ and English under /en/. Pages are Next.js; the promotion's title, dates and
// conditions are in the RSC stream, the conditions as text rows ("39:T1824,<h4>…")
// that the page data points to ("$39"). The sitemap has no lastmod; slugs carry the
// month ("…-oct26-…"), so a page is read once.
type CardX struct{}

const cardxBase = "https://www.cardx.co.th"

func (x *CardX) Bank() string { return "cardx" }

func (x *CardX) List(ctx context.Context, c *fetch.Client) ([]Listing, error) {
	b, err := c.Get(ctx, cardxBase+"/sitemap.xml")
	if err != nil {
		return nil, err
	}
	var set urlSet
	if err := xml.Unmarshal(b, &set); err != nil {
		return nil, fmt.Errorf("cardx sitemap: %w", err)
	}
	return cardxListings(set), nil
}

var cardxPromoRe = regexp.MustCompile(`^/(th-TH|en)/credit-card/promotion/([^/]+)/?$`)

func cardxListings(set urlSet) []Listing {
	byID := map[string]*Listing{}
	var order []string
	for _, u := range set.URLs {
		pu, err := url.Parse(u.Loc)
		if err != nil {
			continue
		}
		m := cardxPromoRe.FindStringSubmatch(pu.Path)
		if m == nil {
			continue
		}
		l := byID[m[2]]
		if l == nil {
			l = &Listing{ExternalID: m[2], Version: m[2], Category: mapCategory(m[2])}
			byID[m[2]] = l
			order = append(order, m[2])
		}
		if m[1] == "en" {
			l.URLEN = u.Loc
		} else {
			l.URLTH = u.Loc
		}
	}
	out := make([]Listing, 0, len(order))
	for _, id := range order {
		if l := byID[id]; l.URLTH != "" {
			out = append(out, *l)
		}
	}
	return out
}

func (x *CardX) Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error) {
	b, err := c.Get(ctx, l.URLTH)
	if err != nil {
		return Detail{}, err
	}
	d, img, err := parseCardXPage(string(b))
	if err != nil {
		return Detail{}, fmt.Errorf("%s: %w", l.URLTH, err)
	}
	d.ImageURL = img
	return d, nil
}

var (
	cardxPageRe   = regexp.MustCompile(`"page":\{"title":\{"th":("(?:[^"\\]|\\.)*"),"en":("(?:[^"\\]|\\.)*")\},"effectiveDate":"([^"]+)","expireDate":"([^"]+)","og_image":\{"url":"([^"]+)"`)
	cardxDetailRe = regexp.MustCompile(`"detail":\{"th":"\$([0-9a-f]+)"`)
	cardxTRowRe   = regexp.MustCompile(`(?m)^([0-9a-f]+):T([0-9a-f]+),`)
)

func parseCardXPage(page string) (Detail, string, error) {
	var b strings.Builder
	for _, m := range ktcChunkRe.FindAllStringSubmatch(page, -1) { // same Next.js stream format as KTC
		var s string
		if json.Unmarshal([]byte(`"`+m[1]+`"`), &s) == nil {
			b.WriteString(s)
		}
	}
	rsc := b.String()
	m := cardxPageRe.FindStringSubmatch(rsc)
	if m == nil {
		return Detail{}, "", fmt.Errorf("no promotion data in page")
	}
	d := Detail{TitleTH: jsonString(m[1]), TitleEN: jsonString(m[2])}
	if d.TitleEN == d.TitleTH {
		d.TitleEN = "" // CardX repeats the Thai title when there is no English
	}
	if t, ok := cardxDate(m[3]); ok {
		d.StartAt = &t
	}
	if t, ok := cardxDate(m[4]); ok {
		end := t.Add(24*time.Hour - time.Minute)
		d.EndAt = &end
	}

	rows := cardxTextRows(rsc)
	var parts []string
	for _, r := range cardxDetailRe.FindAllStringSubmatch(rsc, -1) {
		if s, ok := rows[r[1]]; ok {
			parts = append(parts, s)
		}
	}
	d.TextTH = HTMLText(strings.Join(parts, "\n"))
	return d, m[5], nil
}

// cardxDate reads the CMS date (midnight UTC standing for a Thai calendar day).
func cardxDate(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, config.Thai), true
}

// cardxTextRows reads "<id>:T<hex byte length>,<text>" rows.
func cardxTextRows(rsc string) map[string]string {
	rows := map[string]string{}
	for _, loc := range cardxTRowRe.FindAllStringSubmatchIndex(rsc, -1) {
		n, err := strconv.ParseInt(rsc[loc[4]:loc[5]], 16, 64)
		start := loc[1]
		if err != nil || start+int(n) > len(rsc) {
			continue
		}
		rows[rsc[loc[2]:loc[3]]] = rsc[start : start+int(n)]
	}
	return rows
}
