package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/fetch"
)

// TTB serves Next.js pages with the data in __NEXT_DATA__: the credit card promotion
// page lists sub-categories, each sub-category pages through its promotions (?page=N,
// 8 per page), and each promotion page carries Thai and English detail HTML together.
type TTB struct{}

const ttbBase = "https://www.ttbbank.com"

func (t *TTB) Bank() string { return "ttb" }

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func nextData(page []byte, into any) error {
	m := nextDataRe.FindSubmatch(page)
	if m == nil {
		return fmt.Errorf("no __NEXT_DATA__")
	}
	return json.Unmarshal(m[1], into)
}

type ttbItem struct {
	ID           int    `json:"id"`
	TitleTH      string `json:"title_th"`
	TitleEN      string `json:"title_en"`
	PeriodTH     string `json:"period_th"`
	IntroTH      string `json:"intro_th"`
	IntroEN      string `json:"intro_en"`
	ImageTH      string `json:"image_desktop_th"`
	PostDate     string `json:"post_date"`
	CanonicalURL string `json:"canonical_url"`
}

type ttbListPage struct {
	Props struct {
		PageProps struct {
			Main []struct {
				Slug    string `json:"slug"`
				TitleTH string `json:"title_th"`
			} `json:"mainsubcategoryData"`
			Contents struct {
				Contents    []ttbItem `json:"contents"`
				TotalPages  int       `json:"total_pages"`
				CurrentPage int       `json:"current_page"`
			} `json:"contents"`
		} `json:"pageProps"`
	} `json:"props"`
}

func (t *TTB) List(ctx context.Context, c *fetch.Client) ([]Listing, error) {
	body, err := c.Get(ctx, ttbBase+"/th/promotion/credit-card")
	if err != nil {
		return nil, err
	}
	var root ttbListPage
	if err := nextData(body, &root); err != nil {
		return nil, fmt.Errorf("ttb: %w", err)
	}
	seen := map[int]bool{}
	var out []Listing
	for _, sub := range root.Props.PageProps.Main {
		for page := 1; page <= 50; page++ {
			u := fmt.Sprintf("%s/th/promotion/credit-card/%s?page=%d", ttbBase, sub.Slug, page)
			b, err := c.Get(ctx, u)
			if err != nil {
				return nil, err
			}
			var p ttbListPage
			if err := nextData(b, &p); err != nil {
				return nil, fmt.Errorf("ttb %s: %w", u, err)
			}
			for _, it := range p.Props.PageProps.Contents.Contents {
				if seen[it.ID] {
					continue
				}
				seen[it.ID] = true
				out = append(out, ttbListing(it, sub.Slug))
			}
			cp := p.Props.PageProps.Contents
			if cp.CurrentPage >= cp.TotalPages || len(cp.Contents) == 0 {
				break
			}
		}
	}
	return out, nil
}

func ttbListing(it ttbItem, sub string) Listing {
	start, end := ParseThaiPeriod(it.PeriodTH)
	l := Listing{
		ExternalID: strconv.Itoa(it.ID),
		URLTH:      ttbBase + "/th/" + it.CanonicalURL,
		TitleTH:    it.TitleTH,
		TitleEN:    it.TitleEN,
		DescTH:     HTMLText(it.IntroTH),
		DescEN:     HTMLText(it.IntroEN),
		Category:   mapCategory(sub),
		ImageURL:   it.ImageTH,
		StartAt:    start,
		EndAt:      end,
		PeriodText: it.PeriodTH,
	}
	if it.TitleEN != "" {
		l.URLEN = ttbBase + "/en/" + it.CanonicalURL
	}
	h := sha256.Sum256([]byte(it.TitleTH + "|" + it.TitleEN + "|" + it.PeriodTH + "|" + it.IntroTH + "|" + it.PostDate))
	l.Version = hex.EncodeToString(h[:8])
	return l
}

type ttbDetailPage struct {
	Props struct {
		PageProps struct {
			Content struct {
				TitleTH   string `json:"title_th"`
				TitleEN   string `json:"title_en"`
				DetailTH  string `json:"detail_th"`
				DetailEN  string `json:"detail_en"`
				SeoDescTH string `json:"seo_description_th"`
				SeoDescEN string `json:"seo_description_en"`
				StartDate string `json:"start_date"`
				EndDate   string `json:"end_date"`
				Payload   struct {
					Components []ttbComponent `json:"components"`
				} `json:"payload"`
			} `json:"content"`
		} `json:"pageProps"`
	} `json:"props"`
}

func (t *TTB) Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error) {
	body, err := c.Get(ctx, l.URLTH)
	if err != nil {
		return Detail{}, err
	}
	return parseTTBDetail(body)
}

func parseTTBDetail(body []byte) (Detail, error) {
	var p ttbDetailPage
	if err := nextData(body, &p); err != nil {
		return Detail{}, fmt.Errorf("ttb detail: %w", err)
	}
	ct := p.Props.PageProps.Content
	d := Detail{
		TitleTH: ct.TitleTH, TitleEN: ct.TitleEN,
		DescTH: ct.SeoDescTH, DescEN: ct.SeoDescEN,
		TextTH: HTMLText(ct.DetailTH), TextEN: HTMLText(ct.DetailEN),
	}
	if d.TextTH == "" {
		d.TextTH = componentText(ct.Payload.Components, func(t ttbText) string { return t.TH })
	}
	if d.TextEN == "" {
		d.TextEN = componentText(ct.Payload.Components, func(t ttbText) string { return t.EN })
	}
	if t, err := time.Parse(time.RFC3339, ct.StartDate); err == nil {
		d.StartAt = &t
	}
	if t, err := time.Parse(time.RFC3339, ct.EndDate); err == nil {
		d.EndAt = &t
	}
	return d, nil
}

// ttbComponent is one block of the page builder newer ttb pages use. Only the top-level
// components are read: payload.payload holds a copy.
type ttbComponent struct {
	Data struct {
		Title   ttbText `json:"title"`
		Content ttbText `json:"content"`
	} `json:"data"`
	Components []ttbComponent `json:"components"`
}

type ttbText struct {
	TH string `json:"th"`
	EN string `json:"en"`
}

// UnmarshalJSON accepts a {"th","en"} object and ignores anything else.
func (t *ttbText) UnmarshalJSON(b []byte) error {
	var v struct{ TH, EN string }
	if json.Unmarshal(b, &v) == nil {
		t.TH, t.EN = v.TH, v.EN
	}
	return nil
}

// componentText joins the titles and contents of the components in page order.
func componentText(cs []ttbComponent, lang func(ttbText) string) string {
	var parts []string
	var walk func([]ttbComponent)
	walk = func(cs []ttbComponent) {
		for _, c := range cs {
			for _, s := range []string{lang(c.Data.Title), lang(c.Data.Content)} {
				if s != "" {
					parts = append(parts, s)
				}
			}
			walk(c.Components)
		}
	}
	walk(cs)
	return HTMLText(strings.Join(parts, "\n"))
}
