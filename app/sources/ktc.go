package sources

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/fetch"
)

// KTC lists every promotion page (current and old) in its sitemap with a lastmod, so
// the list costs a dozen small XML files and the crawler only opens pages that changed.
// Each page is a Next.js app; the promotion's data and conditions are in the RSC stream
// (self.__next_f.push) inside the HTML. robots.txt allows /promotion/ without a query.
type KTC struct {
	MaxAge time.Duration // skip pages not changed in this long (old promotions)
}

const ktcBase = "https://www.ktc.co.th"

func (k *KTC) Bank() string { return "ktc" }

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type urlSet struct {
	URLs []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
}

var ktcPromoSitemapRe = regexp.MustCompile(`/sitemap-promotions-\d+\.xml$`)

func (k *KTC) List(ctx context.Context, c *fetch.Client) ([]Listing, error) {
	body, err := c.Get(ctx, ktcBase+"/sitemap.xml")
	if err != nil {
		return nil, err
	}
	var idx sitemapIndex
	if err := xml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("ktc sitemap index: %w", err)
	}
	sets := make([]urlSet, 0, len(idx.Sitemaps))
	for _, s := range idx.Sitemaps {
		if !ktcPromoSitemapRe.MatchString(s.Loc) {
			continue
		}
		b, err := c.Get(ctx, s.Loc)
		if err != nil {
			return nil, err
		}
		var set urlSet
		if err := xml.Unmarshal(b, &set); err != nil {
			return nil, fmt.Errorf("ktc %s: %w", s.Loc, err)
		}
		sets = append(sets, set)
	}
	return k.listingsFrom(sets, time.Now()), nil
}

// listingsFrom pairs the Thai and English URL of each promotion and drops old ones.
func (k *KTC) listingsFrom(sets []urlSet, now time.Time) []Listing {
	byPath := map[string]*Listing{}
	var order []string
	for _, set := range sets {
		for _, u := range set.URLs {
			pu, err := url.Parse(u.Loc)
			if err != nil {
				continue
			}
			path := strings.TrimSuffix(pu.Path, "/")
			en := strings.HasPrefix(path, "/en/")
			path = strings.TrimPrefix(path, "/en")
			// /promotion/<category>/.../<slug>: at least category + slug.
			if strings.Count(path, "/") < 3 || !strings.HasPrefix(path, "/promotion/") {
				continue
			}
			if mod, err := time.Parse(time.RFC3339, u.LastMod); err == nil && k.MaxAge > 0 && now.Sub(mod) > k.MaxAge {
				continue
			}
			l := byPath[path]
			if l == nil {
				l = &Listing{ExternalID: path, Category: mapCategory(strings.Split(path, "/")[2])}
				byPath[path] = l
				order = append(order, path)
			}
			if en {
				l.URLEN = ktcBase + "/en" + path
			} else {
				l.URLTH = ktcBase + path
			}
			if u.LastMod > l.Version {
				l.Version = u.LastMod
			}
		}
	}
	out := make([]Listing, 0, len(order))
	for _, p := range order {
		if l := byPath[p]; l.URLTH != "" {
			out = append(out, *l)
		}
	}
	return out
}

func (k *KTC) Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error) {
	body, err := c.Get(ctx, l.URLTH)
	if err != nil {
		return Detail{}, err
	}
	th, err := parseKTCPage(string(body))
	if err != nil {
		return Detail{}, fmt.Errorf("%s: %w", l.URLTH, err)
	}
	d := Detail{
		TitleTH: th.title, DescTH: th.desc, TextTH: th.text,
		StartAt: th.start, EndAt: th.end, NeedsRegister: th.register,
	}
	if l.URLEN != "" {
		if b, err := c.Get(ctx, l.URLEN); err == nil {
			if en, err := parseKTCPage(string(b)); err == nil {
				d.TitleEN, d.DescEN, d.TextEN = en.title, en.desc, en.text
			}
		}
	}
	return d, nil
}

type ktcPage struct {
	title, desc, text string
	start, end        *time.Time
	register          *bool
}

var (
	ktcChunkRe    = regexp.MustCompile(`self\.__next_f\.push\(\[1,"((?:[^"\\]|\\.)*)"\]\)`)
	ktcDatesRe    = regexp.MustCompile(`"startDate":"\$?D?([0-9T:.\-]+Z)","endDate":"\$?D?([0-9T:.\-]+Z)","meta":\{"title":("(?:[^"\\]|\\.)*"|null),"description":("(?:[^"\\]|\\.)*"|null)`)
	ktcRegisterRe = regexp.MustCompile(`"containRegister":(true|false)`)
	ktcTextRe     = regexp.MustCompile(`"text":"((?:[^"\\]|\\.)*)"`)
)

// The page ends its text with the reCAPTCHA notice; nothing after it is the promotion.
const ktcTextEnd = "This site is protected by reCAPTCHA"

func parseKTCPage(page string) (ktcPage, error) {
	var b strings.Builder
	for _, m := range ktcChunkRe.FindAllStringSubmatch(page, -1) {
		var s string
		if json.Unmarshal([]byte(`"`+m[1]+`"`), &s) == nil {
			b.WriteString(s)
		}
	}
	rsc := b.String()
	at := strings.Index(rsc, `"detail":{"id":`)
	if at < 0 {
		return ktcPage{}, fmt.Errorf("no promotion detail in page")
	}
	var p ktcPage
	if m := ktcDatesRe.FindStringSubmatch(rsc[at:]); m != nil {
		if t, err := time.Parse(time.RFC3339, m[1]); err == nil {
			p.start = &t
		}
		if t, err := time.Parse(time.RFC3339, m[2]); err == nil {
			p.end = &t
		}
		p.title = jsonString(m[3])
		p.desc = jsonString(m[4])
	}
	if ms := ktcRegisterRe.FindAllStringSubmatchIndex(rsc[:at], -1); len(ms) > 0 {
		last := ms[len(ms)-1]
		v := rsc[last[2]:last[3]] == "true"
		p.register = &v
	}
	lines := make([]string, 0, 64)
	for _, m := range ktcTextRe.FindAllStringSubmatch(rsc[at:], -1) {
		var s string
		if json.Unmarshal([]byte(`"`+m[1]+`"`), &s) != nil {
			continue
		}
		if strings.Contains(s, ktcTextEnd) {
			break
		}
		lines = append(lines, s)
	}
	p.text = cleanLines(lines)
	return p, nil
}

// jsonString decodes a JSON string literal ("" for null or bad input).
func jsonString(lit string) string {
	var s string
	if err := json.Unmarshal([]byte(lit), &s); err != nil {
		return ""
	}
	return s
}
