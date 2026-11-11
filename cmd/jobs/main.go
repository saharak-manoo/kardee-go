// Command jobs crawls the banks on a schedule (CRAWL_CRON, Thai time).
//
//	jobs                 run on the schedule
//	jobs -once           crawl every bank once and exit
//	jobs -once -bank ttb crawl one bank once
//	jobs -renormalize     extract conditions, cards and merchants again for every promotion
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/robfig/cron/v3"

	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/crawler"
	"github.com/saharak/kardee-go/app/database"
	"github.com/saharak/kardee-go/app/fetch"
	"github.com/saharak/kardee-go/app/sources"
)

func main() {
	once := flag.Bool("once", false, "crawl once and exit")
	bank := flag.String("bank", "", "only this source (ktc, ttb, krungsri, kbank, jcb, cardx, aeon)")
	renorm := flag.Bool("renormalize", false, "normalize every promotion again and exit")
	flag.Parse()
	if err := run(*once, *bank, *renorm); err != nil {
		log.Fatal(err)
	}
}

func run(once bool, bank string, renorm bool) error {
	cfg := config.Load()
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	if renorm {
		n, err := crawler.Renormalize(db, cat)
		log.Printf("[jobs] renormalized %d promotions", n)
		return err
	}
	cr := &crawler.Crawler{DB: db, Client: fetch.New(cfg.UserAgent, cfg.CrawlDelay), DetailLimit: cfg.DetailLimit, Catalog: cat}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var running sync.Mutex
	crawlAll := func() {
		if !running.TryLock() {
			log.Print("[jobs] previous crawl still running, skipping")
			return
		}
		defer running.Unlock()
		// Banks are different hosts, so they run side by side; each host gets one
		// request at a time from the fetch client.
		var wg sync.WaitGroup
		for _, src := range sources.All(cfg.KTCMaxAgeDays) {
			if bank != "" && src.Bank() != bank {
				continue
			}
			wg.Add(1)
			go func(s sources.Source) {
				defer wg.Done()
				cr.Run(ctx, s)
			}(src)
		}
		wg.Wait()
	}

	if once {
		crawlAll()
		return nil
	}

	c := cron.New(cron.WithLocation(config.Thai))
	if _, err := c.AddFunc(cfg.CrawlCron, crawlAll); err != nil {
		return fmt.Errorf("CRAWL_CRON %q: %w", cfg.CrawlCron, err)
	}
	c.Start()
	log.Printf("[jobs] crawling on %q (Thai time)", cfg.CrawlCron)
	<-ctx.Done()
	<-c.Stop().Done()
	return nil
}
