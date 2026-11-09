// Package sources reads credit card promotions from each bank's public website.
//
// Rules for every source: only pages robots.txt allows, the polite fetch.Client
// (one request at a time, a pause between them), and no getting around bot protection.
// A bank that blocks automated reading is left out, not worked around.
package sources

import (
	"context"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/fetch"
)

// Listing is what a bank's promotion list gives for one promotion.
type Listing struct {
	ExternalID string // stable per bank: a path or the bank's id
	URLTH      string
	URLEN      string
	TitleTH    string
	TitleEN    string
	DescTH     string
	DescEN     string
	Category   string // our key (models.Categories)
	Cards      []string
	ImageURL   string
	StartAt    *time.Time
	EndAt      *time.Time
	PeriodText string
	// Version changes when the bank edits the promotion (lastmod, modified date...).
	// The crawler reads the detail page again only when it changes.
	Version string
}

// Detail is what the promotion's own page adds.
type Detail struct {
	TitleTH, TitleEN string
	DescTH, DescEN   string
	TextTH, TextEN   string // conditions as plain text
	StartAt, EndAt   *time.Time
	NeedsRegister    *bool
	PromoCode        string
}

// Source is one bank.
type Source interface {
	Bank() string
	List(ctx context.Context, c *fetch.Client) ([]Listing, error)
	Detail(ctx context.Context, c *fetch.Client, l Listing) (Detail, error)
}

// All returns every source that is switched on.
func All(ktcMaxAgeDays int) []Source {
	return []Source{
		&KTC{MaxAge: time.Duration(ktcMaxAgeDays) * 24 * time.Hour},
		&TTB{},
		&Krungsri{},
		&KBank{},
		&JCB{},
	}
}

var (
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	dropRe  = regexp.MustCompile(`(?is)<(script|style|noscript|svg)[^>]*>.*?</(script|style|noscript|svg)>`)
	blockRe = regexp.MustCompile(`(?i)<(br|/p|/div|/li|/h[1-6]|/tr|/td)[^>]*>`)
	spaceRe = regexp.MustCompile(`[ \t\x{00a0}\x{200b}]+`)
)

// HTMLText turns HTML into plain text, one block per line, no empty lines.
func HTMLText(s string) string {
	s = dropRe.ReplaceAllString(s, " ")
	s = blockRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return cleanLines(strings.Split(s, "\n"))
}

func cleanLines(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(spaceRe.ReplaceAllString(l, " "))
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
