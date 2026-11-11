// Command api serves Kardee promotions over HTTP.
package main

import (
	"log"

	"github.com/gofiber/fiber/v3"

	"github.com/saharak/kardee-go/app/api"
	"github.com/saharak/kardee-go/app/auth"
	"github.com/saharak/kardee-go/app/catalog"
	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/database"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	cat, err := catalog.Load()
	if err != nil {
		log.Fatal(err)
	}
	app := fiber.New(fiber.Config{AppName: "kardee-api"})
	srv := &api.Server{DB: db, Catalog: cat}
	if cfg.LINELoginChannelID != "" {
		srv.Verifier = &auth.LINEVerifier{ChannelID: cfg.LINELoginChannelID}
	}
	srv.Routes(app)
	log.Fatal(app.Listen(":" + cfg.Port))
}
