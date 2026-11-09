package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/fetch"
)

// JCB publishes its card network's own offers on specialoffers.jcb. The Thailand list
// is a plain JSON file the page loads (no robots.txt on the host, no bot wall); each
// offer has an HTML page with the dates and conditions. Offers apply to JCB cards of
// any bank, so the bank key is the network ("jcb").
type JCB struct{}

const (
	jcbBase     = "https://www.specialoffers.jcb"
	jcbListURL  = jcbBase + "/th/offers/output/d/southeast-and-south-asia/thailand/ajax.json"
	jcbImageURL = "https://branddb.jcb.jp/images/merchant/"
)

func (j *JCB) Bank() string { return "jcb" }

type jcbEntry struct {
	Title string `json:"title"`
	Link  string `json:"link"`
	Image string `json:"image"`
	Cat1  string `json:"cat1"`
	Cat2  string `json:"cat2"`
	Disc  string `json:"disc"`
	Base1 string `json:"base1"`
}

func (j *JCB) List(ctx context.Context, c *fetch.Client) ([]Listing, error) {
	b, err := c.Get(ctx, jcbListURL)
	if err != nil {
		return nil, err
	}
	return jcbListings(b)
}

// jcbCategories maps JCB's English category keys (its Thai names trip mapCategory:
// "พักผ่อน" contains "ผ่อน").
var jcbCategories = map[string]string{
	"restaurant":    "dining",
	"relaxation":    "health",
	"retail_store":  "shopping",
	"entertainment": "entertain",
	"hotel":         "travel",
	"travel":        "travel",
}

func jcbCategory(base string) string {
	if c, ok := jcbCategories[base]; ok {
		return c
	}
	return mapCategory(base)
}

var jcbIDRe = regexp.MustCompile(`/detail/([^/]+)/?$`)

func jcbListings(b []byte) ([]Listing, error) {
	var root struct {
		Entries []jcbEntry `json:"entries"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("jcb list: %w", err)
	}
	out := make([]Listing, 0, len(root.Entries))
	for _, e := range root.Entries {
		m := jcbIDRe.FindStringSubmatch(e.Link)
		if m == nil {
			continue
		}
		l := Listing{
			ExternalID: m[1],
			URLTH:      jcbBase + e.Link,
			TitleTH:    e.Title,
			DescTH:     strings.TrimSpace(e.Disc + " · " + strings.Trim(e.Cat1+" "+e.Cat2, " ")),
			Category:   jcbCategory(e.Base1),
		}
		if e.Image != "" {
			l.ImageURL = jcbImageURL + e.Image
		}
		h := sha256.Sum256([]byte(e.Title + "|" + e.Disc + "|" + e.Link + "|" + e.Image))
		l.Version = hex.EncodeToString(h[:8])
		out = append(out, l)
	}
	return out, nil
}

func (j *JCB) Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error) {
	b, err := c.Get(ctx, l.URLTH)
	if err != nil {
		return Detail{}, err
	}
	return parseJCBDetail(b), nil
}

var jcbValidRe = regexp.MustCompile(`Valid\s*:\s*(\d{4}-\d{2}-\d{2})\s*-\s*(\d{4}-\d{2}-\d{2})`)

// The offer's text sits between its "Valid :" line and the back link.
const jcbTextEnd = "Back to the previous page"

func parseJCBDetail(b []byte) Detail {
	text := HTMLText(string(b))
	var d Detail
	if m := jcbValidRe.FindStringSubmatch(text); m != nil {
		if t, err := time.ParseInLocation("2006-01-02", m[1], config.Thai); err == nil {
			d.StartAt = &t
		}
		if t, err := time.ParseInLocation("2006-01-02", m[2], config.Thai); err == nil {
			end := t.Add(24*time.Hour - time.Minute)
			d.EndAt = &end
		}
	}
	if loc := jcbValidRe.FindStringIndex(text); loc != nil {
		text = text[loc[1]:]
	}
	if i := strings.Index(text, jcbTextEnd); i >= 0 {
		text = text[:i]
	}
	d.TextTH = strings.TrimSpace(text)
	return d
}
