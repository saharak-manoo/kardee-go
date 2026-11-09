package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/fetch"
)

// Krungsri credit cards live on krungsricard.com (krungsri.com sits behind a bot wall,
// so it is not used). The promotion page embeds every promotion as
// `var promotionData = [...]` (~560 items: title, card types, categories, period,
// modified date). Detail pages are campaign landing pages; their text has the offer,
// the merchants and the dates, but the full conditions are often in a PDF we don't read.
// robots.txt allows /th/promotion and disallows /en/promotion, so Krungsri is Thai only.
type Krungsri struct{}

const krungsriBase = "https://www.krungsricard.com"

func (k *Krungsri) Bank() string { return "krungsri" }

// looseString takes a JSON string, bool, number or null: Krungsri's data mixes them
// in the same field ("False" in one item, false in the next).
type looseString string

func (l *looseString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*l = looseString(s)
		return nil
	}
	if string(b) == "null" {
		*l = ""
		return nil
	}
	*l = looseString(strings.Trim(string(b), `"`))
	return nil
}

func (l looseString) String() string { return string(l) }

type krungsriItem struct {
	NodeID          looseString `json:"NodeId"`
	Hidden          looseString `json:"HiddenPromotion"`
	Title           looseString `json:"Title"`
	OGDescription   looseString `json:"OG_Description"`
	TypePromotion   looseString `json:"TypePromotion"`
	CardType        looseString `json:"CardType"`
	TextDatetime    looseString `json:"Text_datetime"`
	PublishTo       looseString `json:"DocumentPublishTo"`
	Modified        looseString `json:"DocumentModifiedWhen"`
	AbsoluteURL     looseString `json:"AbsoluteUrl"`
	BannerPC        looseString `json:"BannerPC"`
	DocumentCulture looseString `json:"Culture"`
}

func (k *Krungsri) List(ctx context.Context, c *fetch.Client) ([]Listing, error) {
	body, err := c.Get(ctx, krungsriBase+"/th/promotion")
	if err != nil {
		return nil, err
	}
	return parseKrungsriList(string(body))
}

func parseKrungsriList(page string) ([]Listing, error) {
	const marker = "var promotionData = "
	at := strings.Index(page, marker)
	if at < 0 {
		return nil, fmt.Errorf("krungsri: promotionData not found")
	}
	var items []krungsriItem
	if err := json.NewDecoder(strings.NewReader(page[at+len(marker):])).Decode(&items); err != nil {
		return nil, fmt.Errorf("krungsri: %w", err)
	}
	out := make([]Listing, 0, len(items))
	for _, it := range items {
		if strings.EqualFold(it.Hidden.String(), "true") || it.AbsoluteURL.String() == "" || it.AbsoluteURL.String() == "None" {
			continue
		}
		start, end := ParseThaiPeriod(it.TextDatetime.String())
		if end == nil {
			if t, err := time.ParseInLocation("2006-01-02T15:04:05", it.PublishTo.String(), config.Thai); err == nil && t.Year() > 2000 {
				end = &t
			}
		}
		l := Listing{
			ExternalID: it.NodeID.String(),
			URLTH:      it.AbsoluteURL.String(),
			TitleTH:    strings.TrimSpace(it.Title.String()),
			DescTH:     strings.TrimSpace(it.OGDescription.String()),
			Category:   mapCategory(strings.SplitN(it.TypePromotion.String(), "|", 2)[0]),
			StartAt:    start,
			EndAt:      end,
			PeriodText: it.TextDatetime.String(),
			Version:    it.Modified.String(),
		}
		if it.BannerPC.String() != "" && it.BannerPC.String() != "None" {
			l.ImageURL = krungsriBase + it.BannerPC.String()
		}
		for _, card := range strings.Split(it.CardType.String(), "|") {
			if card = strings.TrimSpace(card); card != "" && card != "None" {
				l.Cards = append(l.Cards, card)
			}
		}
		out = append(out, l)
	}
	return out, nil
}

// Text after these lines is the site's footer and related promotions, not this one.
var krungsriStops = []string{"บัตรเครดิตที่ร่วมรายการ", "รวมโปรโมชันบัตรเครดิต", "เกี่ยวกับเรา"}

func (k *Krungsri) Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error) {
	body, err := c.Get(ctx, l.URLTH)
	if err != nil {
		return Detail{}, err
	}
	return Detail{TextTH: krungsriText(string(body), l.TitleTH)}, nil
}

// krungsriText keeps the page text from the promotion's title to the first stop line.
func krungsriText(page, title string) string {
	lines := strings.Split(HTMLText(page), "\n")
	from := 0
	key := strings.TrimSuffix(strings.TrimSpace(title), "*")
	for i, l := range lines {
		if key != "" && strings.Contains(l, key) {
			from = i
			break
		}
	}
	to := len(lines)
	for i := from + 1; i < len(lines); i++ {
		stop := false
		for _, s := range krungsriStops {
			if strings.HasPrefix(lines[i], s) {
				stop = true
			}
		}
		if stop {
			to = i
			break
		}
	}
	return strings.Join(lines[from:to], "\n")
}
