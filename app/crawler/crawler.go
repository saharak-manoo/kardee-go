// Package crawler runs one bank's source: list, save, read detail pages that changed,
// extract conditions, and mark promotions the bank no longer lists.
package crawler

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/saharak/kardee-go/app/fetch"
	"github.com/saharak/kardee-go/app/models"
	"github.com/saharak/kardee-go/app/normalize"
	"github.com/saharak/kardee-go/app/sources"
)

type Crawler struct {
	DB          *gorm.DB
	Client      *fetch.Client
	DetailLimit int // detail pages per run; the rest wait for the next run
}

// Run crawls one bank and records the run. It returns the run even on error.
func (c *Crawler) Run(ctx context.Context, src sources.Source) models.CrawlRun {
	run := models.CrawlRun{Bank: src.Bank(), StartedAt: time.Now()}
	defer func() {
		run.Duration = time.Since(run.StartedAt).Seconds()
		if err := c.DB.Create(&run).Error; err != nil {
			log.Printf("[crawl] %s: save run: %v", src.Bank(), err)
		}
	}()

	listings, err := src.List(ctx, c.Client)
	if err != nil {
		run.Error = err.Error()
		if errors.Is(err, sources.ErrSkipped) {
			log.Printf("[crawl] %s skipped: %v", src.Bank(), err)
		} else {
			log.Printf("[crawl] %s list failed: %v", src.Bank(), err)
		}
		return run
	}
	run.Listed = len(listings)

	var existing []models.Promotion
	if err := c.DB.Where("bank = ?", src.Bank()).Find(&existing).Error; err != nil {
		run.Error = err.Error()
		return run
	}
	byID := make(map[string]*models.Promotion, len(existing))
	for i := range existing {
		byID[existing[i].ExternalID] = &existing[i]
	}

	now := time.Now()
	seen := make(map[string]bool, len(listings))
	var todo []*models.Promotion
	saved := make([]*models.Promotion, 0, len(listings))
	for _, l := range listings {
		if seen[l.ExternalID] {
			continue
		}
		seen[l.ExternalID] = true
		p := byID[l.ExternalID]
		if p == nil {
			p = &models.Promotion{Bank: src.Bank(), ExternalID: l.ExternalID}
			run.New++
		}
		fromListing(p, l)
		p.LastSeenAt = now
		p.Gone = false
		if p.DetailFetchedAt == nil || p.DetailVersion != p.SourceVersion {
			todo = append(todo, p)
		}
		saved = append(saved, p)
	}

	// New promotions first, then the ones changed longest ago.
	sort.SliceStable(todo, func(i, j int) bool {
		if (todo[i].ID == 0) != (todo[j].ID == 0) {
			return todo[i].ID == 0
		}
		return todo[i].UpdatedAt.Before(todo[j].UpdatedAt)
	})
	if c.DetailLimit > 0 && len(todo) > c.DetailLimit {
		todo = todo[:c.DetailLimit]
	}
	for _, p := range todo {
		if ctx.Err() != nil {
			break
		}
		d, err := src.Detail(ctx, c.Client, listingOf(p))
		if err != nil {
			log.Printf("[crawl] %s %s: %v", src.Bank(), p.ExternalID, err)
			if errors.Is(err, fetch.ErrBlocked) {
				run.Error = err.Error()
				break // stop knocking once the site says no
			}
			continue
		}
		fromDetail(p, d)
		t := time.Now()
		p.DetailFetchedAt = &t
		p.DetailVersion = p.SourceVersion
		run.Details++
	}

	for _, p := range saved {
		normalize.Apply(p, normalize.Extract(p.TitleTH+"\n"+p.DescriptionTH, p.DetailTH+"\n"+p.DetailEN))
		if err := c.DB.Save(p).Error; err != nil {
			log.Printf("[crawl] %s %s: save: %v", src.Bank(), p.ExternalID, err)
		}
	}

	// Gone: listed before, not now. Only after a list that returned something.
	if len(listings) > 0 {
		var gone []string
		for id, p := range byID {
			if !seen[id] && !p.Gone {
				gone = append(gone, id)
			}
		}
		if len(gone) > 0 {
			c.DB.Model(&models.Promotion{}).Where("bank = ? AND external_id IN ?", src.Bank(), gone).Update("gone", true)
			run.Gone = len(gone)
		}
	}
	log.Printf("[crawl] %s: listed %d, new %d, details %d, gone %d in %s",
		src.Bank(), run.Listed, run.New, run.Details, run.Gone, time.Since(run.StartedAt).Round(time.Second))
	return run
}

func fromListing(p *models.Promotion, l sources.Listing) {
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&p.URLTH, l.URLTH)
	set(&p.URLEN, l.URLEN)
	set(&p.TitleTH, l.TitleTH)
	set(&p.TitleEN, l.TitleEN)
	set(&p.DescriptionTH, l.DescTH)
	set(&p.DescriptionEN, l.DescEN)
	set(&p.Category, l.Category)
	set(&p.ImageURL, l.ImageURL)
	set(&p.PeriodText, l.PeriodText)
	if len(l.Cards) > 0 {
		p.Cards = strings.Join(l.Cards, "|")
	}
	if l.StartAt != nil {
		p.StartAt = l.StartAt
	}
	if l.EndAt != nil {
		p.EndAt = l.EndAt
	}
	p.SourceVersion = l.Version
}

func fromDetail(p *models.Promotion, d sources.Detail) {
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&p.TitleTH, d.TitleTH)
	set(&p.TitleEN, d.TitleEN)
	set(&p.DescriptionTH, d.DescTH)
	set(&p.DescriptionEN, d.DescEN)
	set(&p.DetailTH, d.TextTH)
	set(&p.DetailEN, d.TextEN)
	// The detail page's own dates are more exact than the list's period text.
	if d.StartAt != nil {
		p.StartAt = d.StartAt
	}
	if d.EndAt != nil {
		p.EndAt = d.EndAt
	}
	if d.PromoCode != "" {
		p.PromoCode = d.PromoCode
	}
}

func listingOf(p *models.Promotion) sources.Listing {
	return sources.Listing{
		ExternalID: p.ExternalID, URLTH: p.URLTH, URLEN: p.URLEN,
		TitleTH: p.TitleTH, TitleEN: p.TitleEN, Version: p.SourceVersion,
	}
}
